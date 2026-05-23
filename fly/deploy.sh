#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

info() { printf '\033[0;32m==> \033[0m%s\n' "$*"; }
warn() { printf '\033[1;33mWARNING:\033[0m %s\n' "$*"; }
die()  { printf '\033[0;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# ── Step 1: check flyctl ─────────────────────────────────────────────────────
FLY=""
if command -v fly &>/dev/null; then
    FLY="fly"
elif command -v flyctl &>/dev/null; then
    FLY="flyctl"
else
    die "flyctl is not installed.
Install it with:
  curl -L https://fly.io/install.sh | sh
Then re-run this script."
fi

# ── Step 2: check auth ───────────────────────────────────────────────────────
if ! $FLY auth whoami &>/dev/null; then
    info "Not logged in to Fly.io. Starting login..."
    $FLY auth login
fi
info "Logged in as: $($FLY auth whoami)"

# ── Step 3: app name ─────────────────────────────────────────────────────────
printf 'App name [dexmon]: '
read -r APP_NAME
APP_NAME="${APP_NAME:-dexmon}"

# ── Step 4: region ───────────────────────────────────────────────────────────
echo ""
echo "Fly.io regions: https://fly.io/docs/reference/regions/"
printf 'Primary region [iad]: '
read -r REGION
REGION="${REGION:-iad}"

# ── Step 5: create app ───────────────────────────────────────────────────────
info "Checking app '$APP_NAME'..."
if $FLY apps list 2>/dev/null | grep -qE "(^|[[:space:]])${APP_NAME}([[:space:]]|$)"; then
    info "App '$APP_NAME' already exists, skipping creation."
else
    info "Creating app '$APP_NAME'..."
    $FLY apps create "$APP_NAME"
fi

# ── Step 6: create volume ────────────────────────────────────────────────────
info "Checking persistent volume..."
if $FLY volumes list --app "$APP_NAME" 2>/dev/null | grep -q "dexmon_data"; then
    info "Volume 'dexmon_data' already exists, skipping."
else
    info "Creating 1 GB volume 'dexmon_data' in region '$REGION'..."
    $FLY volumes create dexmon_data \
        --size 1 \
        --region "$REGION" \
        --app "$APP_NAME" \
        --yes
fi

# ── Step 7: config file ──────────────────────────────────────────────────────
echo ""
printf 'Path to your config.toml [./config.toml]: '
read -r CONFIG_INPUT
CONFIG_INPUT="${CONFIG_INPUT:-./config.toml}"
CONFIG_PATH="$(cd "$(dirname "$CONFIG_INPUT")" 2>/dev/null && pwd)/$(basename "$CONFIG_INPUT")"
[ -f "$CONFIG_PATH" ] || die "Config file not found: $CONFIG_INPUT"
cd "$PROJECT_ROOT"

# ── Step 8: discover and prompt for secrets ──────────────────────────────────
info "Scanning config.toml for required environment variables..."
VARS=$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*\}' "$CONFIG_PATH" \
    | sed 's/\${//;s/}//' \
    | sort -u) || true

if [ -z "$VARS" ]; then
    warn "No \${VAR} references found in config.toml."
fi

SECRET_ARGS=()
while IFS= read -r VAR; do
    [ -z "$VAR" ] && continue
    printf 'Value for %s: ' "$VAR"
    read -rs VALUE || die "Aborted by user."
    echo ""
    SECRET_ARGS+=("$VAR=$VALUE")
done <<< "$VARS"

# ── Step 9: set secrets ──────────────────────────────────────────────────────
info "Encoding config.toml and setting secrets..."
CONFIG_B64=$(base64 < "$CONFIG_PATH" | tr -d '\n')
$FLY secrets set CONFIG_TOML="$CONFIG_B64" "${SECRET_ARGS[@]}" --app "$APP_NAME"

# ── Step 10: substitute placeholders in fly.toml ─────────────────────────────
FLYTOML="$SCRIPT_DIR/fly.toml"
[ -f "$FLYTOML" ] || die "fly/fly.toml not found. Run this script from the project root."
if grep -q 'DEXMON_APP_NAME' "$FLYTOML"; then
    info "Configuring fly/fly.toml..."
    if [ "$(uname)" = "Darwin" ]; then
        sed -i '' "s|DEXMON_APP_NAME|$APP_NAME|g;s|DEXMON_REGION|$REGION|g" "$FLYTOML"
    else
        sed -i "s|DEXMON_APP_NAME|$APP_NAME|g;s|DEXMON_REGION|$REGION|g" "$FLYTOML"
    fi
fi

# ── Step 11: deploy ──────────────────────────────────────────────────────────
info "Deploying to Fly.io..."
cd "$PROJECT_ROOT"
$FLY deploy --remote-only --config fly/fly.toml

# ── Step 12: post-deploy instructions ────────────────────────────────────────
echo ""
info "Deployment complete!"
echo ""
echo "  App URL: https://$APP_NAME.fly.dev"
echo ""
echo "IMPORTANT: update callback_url in your config.toml:"
echo ""
echo "  [server]"
echo "  callback_url = \"https://$APP_NAME.fly.dev/pushover/callback\""
echo ""
echo "Then run ./fly/update.sh and choose option 1 or 4."
