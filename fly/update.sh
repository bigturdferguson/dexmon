#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

info() { printf '\033[0;32m==> \033[0m%s\n' "$*"; }
die()  { printf '\033[0;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

print_app_state() {
    echo ""
    info "App state:"
    printf '      App:          %s\n' "$APP_NAME"
    if [ -n "${CONFIG_PATH:-}" ] && [ -f "${CONFIG_PATH:-}" ]; then
        local callback_url
        callback_url=$(grep 'callback_url' "$CONFIG_PATH" \
            | head -1 \
            | sed "s/.*= *['\"]//;s/['\"].*//" || true)
        local vars_display
        vars_display=$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*\}' "$CONFIG_PATH" \
            | sed 's/\${//;s/}//' | sort -u | tr '\n' ' ' | sed 's/ $//' || true)
        printf '      Config:       %s\n' "$CONFIG_PATH"
        [ -n "$callback_url" ] && printf '      Callback URL: %s\n' "$callback_url"
        [ -n "$vars_display" ] && printf '      Secrets:      %s\n' "$vars_display"
    else
        printf '      App URL:      https://%s.fly.dev\n' "$APP_NAME"
    fi
    echo ""
}

# ── detect flyctl ─────────────────────────────────────────────────────────────
FLY=""
if command -v fly &>/dev/null; then
    FLY="fly"
elif command -v flyctl &>/dev/null; then
    FLY="flyctl"
else
    die "flyctl is not installed.
Install it with:
  curl -L https://fly.io/install.sh | sh"
fi

# ── check auth ────────────────────────────────────────────────────────────────
if ! $FLY auth whoami &>/dev/null; then
    info "Not logged in to Fly.io. Starting login..."
    $FLY auth login
fi

# ── detect app name from fly.toml ────────────────────────────────────────────
FLYTOML="$SCRIPT_DIR/fly.toml"
APP_NAME=$(awk -F"'" '/^app/{print $2}' "$FLYTOML")
if [ -z "$APP_NAME" ] || [ "$APP_NAME" = "DEXMON_APP_NAME" ]; then
    die "App not configured in fly/fly.toml. Run ./fly/deploy.sh first."
fi
info "Updating app: $APP_NAME"

# ── helpers ───────────────────────────────────────────────────────────────────
get_config_path() {
    printf 'Path to your config.toml [./config.toml]: '
    read -r CONFIG_INPUT
    CONFIG_INPUT="${CONFIG_INPUT:-./config.toml}"
    CONFIG_PATH="$(cd "$(dirname "$CONFIG_INPUT")" 2>/dev/null && pwd)/$(basename "$CONFIG_INPUT")"
    [ -f "$CONFIG_PATH" ] || die "Config file not found: $CONFIG_INPUT"
    info "Config: $CONFIG_PATH"
    export CONFIG_PATH
}

push_config() {
    info "Encoding and uploading config.toml..."
    CONFIG_B64=$(base64 < "$CONFIG_PATH" | tr -d '\n')
    $FLY secrets set CONFIG_TOML="$CONFIG_B64" --app "$APP_NAME"
}

push_secrets() {
    info "Scanning config.toml for environment variables..."
    VARS=$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*\}' "$CONFIG_PATH" \
        | sed 's/\${//;s/}//' \
        | sort -u) || true
    if [ -n "$VARS" ]; then
        info "Secrets to review: $(echo "$VARS" | tr '\n' ' ' | sed 's/ $//')"
    fi
    SECRET_ARGS=()
    while IFS= read -r VAR; do
        [ -z "$VAR" ] && continue
        printf 'Value for %s (leave blank to keep existing): ' "$VAR" >/dev/tty
        read -rs VALUE < /dev/tty || die "Aborted by user."
        printf '\n' >/dev/tty
        if [ -z "$VALUE" ]; then
            info "Skipping $VAR (no value entered)"
            continue
        fi
        SECRET_ARGS+=("$VAR=$VALUE")
    done <<< "$VARS"
    if [ "${#SECRET_ARGS[@]}" -gt 0 ]; then
        $FLY secrets set "${SECRET_ARGS[@]}" --app "$APP_NAME"
    fi
}

push_all() {
    info "Encoding config.toml and scanning for environment variables..."
    CONFIG_B64=$(base64 < "$CONFIG_PATH" | tr -d '\n')
    VARS=$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*\}' "$CONFIG_PATH" \
        | sed 's/\${//;s/}//' \
        | sort -u) || true
    if [ -n "$VARS" ]; then
        info "Secrets to review: $(echo "$VARS" | tr '\n' ' ' | sed 's/ $//')"
    fi
    SECRET_ARGS=()
    while IFS= read -r VAR; do
        [ -z "$VAR" ] && continue
        printf 'Value for %s (leave blank to keep existing): ' "$VAR" >/dev/tty
        read -rs VALUE < /dev/tty || die "Aborted by user."
        printf '\n' >/dev/tty
        if [ -z "$VALUE" ]; then
            info "Skipping $VAR (no value entered)"
            continue
        fi
        SECRET_ARGS+=("$VAR=$VALUE")
    done <<< "$VARS"
    info "Setting all secrets in one call..."
    $FLY secrets set CONFIG_TOML="$CONFIG_B64" "${SECRET_ARGS[@]}" --app "$APP_NAME"
}

do_deploy() {
    info "Deploying..."
    cd "$PROJECT_ROOT"
    $FLY deploy --config fly/fly.toml
}

# ── menu ──────────────────────────────────────────────────────────────────────
echo ""
echo "What would you like to update?"
echo "  1. Config file  (re-encode config.toml, redeploy)"
echo "  2. Secrets      (re-prompt for \${VAR} values, redeploy)"
echo "  3. Code only    (fly deploy)"
echo "  4. Everything   (config + secrets + redeploy)"
echo ""
printf 'Choice [1-4]: '
read -r CHOICE

case "$CHOICE" in
    1) info "Option 1: re-encoding config";          get_config_path; push_config;   do_deploy ;;
    2) info "Option 2: updating secrets";            get_config_path; push_secrets;  do_deploy ;;
    3) info "Option 3: code-only deploy";                                             do_deploy ;;
    4) info "Option 4: updating config and secrets"; get_config_path; push_all;      do_deploy ;;
    *) die "Invalid choice: $CHOICE" ;;
esac

print_app_state
info "Done."
