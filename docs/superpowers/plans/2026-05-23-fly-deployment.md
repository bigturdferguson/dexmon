# Fly.io Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add automated Fly.io deployment via a first-time setup script, an update script, an optional GitHub Actions workflow, and a Dockerfile, then update the README.

**Architecture:** A two-stage Dockerfile builds the binary into an Alpine image. An entrypoint script decodes the `CONFIG_TOML` Fly secret (base64-encoded config.toml) to disk before starting dexmon. `fly/deploy.sh` handles first-time setup end-to-end; `fly/update.sh` handles subsequent config/secret/code changes. A GitHub Actions workflow is included but opt-in (requires adding a `FLY_API_TOKEN` secret to GitHub).

**Tech Stack:** Bash, Docker (multi-stage Alpine), Fly.io (`flyctl`), GitHub Actions (`superfly/flyctl-actions`)

---

## File Structure

```
Dockerfile                          # two-stage build: Go builder → Alpine runtime
.dockerignore                       # exclude .git, docs/, built binary, *.db
fly/
  entrypoint.sh                     # decode CONFIG_TOML → /data/config.toml, exec dexmon
  fly.toml                          # Fly app config template (placeholders substituted by deploy.sh)
  deploy.sh                         # first-time setup: flyctl check, auth, app, volume, secrets, deploy
  update.sh                         # update: config / secrets / code / all
.github/
  workflows/
    fly-deploy.yml                  # optional CI/CD: push to main → fly deploy
README.md                           # replace Fly.io stub with full deployment guide
```

---

### Task 1: Dockerfile, .dockerignore, and entrypoint.sh

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`
- Create: `fly/entrypoint.sh`

**Context:** dexmon uses `modernc.org/sqlite` (pure Go, no CGO). The binary is fully static with `CGO_ENABLED=0`. The runtime image needs `ca-certificates` for HTTPS connections to Dexcom, Pushover, and healthchecks.io. The entrypoint decodes the `CONFIG_TOML` env var (base64) to `/data/config.toml` before starting dexmon. `/data` is the Fly persistent volume mount.

- [ ] **Step 1: Create `Dockerfile`**

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o dexmon .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/dexmon ./dexmon
COPY fly/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh
ENTRYPOINT ["/app/entrypoint.sh"]
```

- [ ] **Step 2: Create `.dockerignore`**

```
.git
docs/
dexmon
*.db
*.env
```

- [ ] **Step 3: Create `fly/entrypoint.sh`**

```sh
#!/bin/sh
set -e

if [ -z "${CONFIG_TOML:-}" ]; then
    echo "ERROR: CONFIG_TOML environment variable is required" >&2
    exit 1
fi

mkdir -p /data
printf '%s' "$CONFIG_TOML" | base64 -d > /data/config.toml

exec /app/dexmon -config /data/config.toml -db /data/dexmon.db
```

- [ ] **Step 4: Make entrypoint.sh executable**

```bash
chmod +x fly/entrypoint.sh
```

- [ ] **Step 5: Verify the Docker build succeeds**

```bash
docker build -t dexmon-test .
```

Expected: build completes, final image is Alpine-based with the dexmon binary.

If Docker is not available locally, skip this step — the build will be verified during deploy.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile .dockerignore fly/entrypoint.sh
git commit -m "feat: add Dockerfile and entrypoint for Fly.io deployment"
```

---

### Task 2: fly.toml

**Files:**
- Create: `fly/fly.toml`

**Context:** `fly.toml` is a template — `DEXMON_APP_NAME` and `DEXMON_REGION` are placeholder strings that `deploy.sh` replaces with `sed` on first run. `auto_stop_machines = false` is critical: dexmon must run continuously for CGM polling; Fly's default behavior stops idle machines. `min_machines_running = 1` keeps exactly one instance alive at all times.

- [ ] **Step 1: Create `fly/fly.toml`**

```toml
app = 'DEXMON_APP_NAME'
primary_region = 'DEXMON_REGION'

[build]

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 1

[[mounts]]
  source = 'dexmon_data'
  destination = '/data'
```

- [ ] **Step 2: Commit**

```bash
git add fly/fly.toml
git commit -m "feat: add fly.toml template for Fly.io app configuration"
```

---

### Task 3: deploy.sh

**Files:**
- Create: `fly/deploy.sh`

**Context:** This script is the complete first-time setup flow. It must handle both `fly` and `flyctl` command names (Fly renamed the binary). The script parses the user's config.toml for `${VAR_NAME}` patterns to discover which secrets are needed — this is intentional so the list is driven by the config, not hardcoded. All secrets are uploaded in a single `fly secrets set` call. `base64 | tr -d '\n'` strips newlines from the encoded output to produce a single-line value safe for env var storage. `sed -i` behaves differently on macOS (`sed -i ''`) vs Linux (`sed -i`), so the OS is detected. The script modifies `fly/fly.toml` in-place only when the placeholder `DEXMON_APP_NAME` is still present (idempotent).

- [ ] **Step 1: Create `fly/deploy.sh`**

```bash
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
if $FLY apps list 2>/dev/null | grep -qw "$APP_NAME"; then
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
cd "$PROJECT_ROOT"
[ -f "$CONFIG_INPUT" ] || die "Config file not found: $CONFIG_INPUT"
CONFIG_PATH="$(cd "$(dirname "$CONFIG_INPUT")" && pwd)/$(basename "$CONFIG_INPUT")"

# ── Step 8: discover and prompt for secrets ──────────────────────────────────
info "Scanning config.toml for required environment variables..."
VARS=$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*\}' "$CONFIG_PATH" \
    | sed 's/\${//;s/}//' \
    | sort -u) || true

if [ -z "$VARS" ]; then
    warn "No \${VAR} references found in config.toml."
fi

SECRET_ARGS=""
while IFS= read -r VAR; do
    [ -z "$VAR" ] && continue
    printf 'Value for %s: ' "$VAR"
    read -rs VALUE
    echo ""
    SECRET_ARGS="$SECRET_ARGS $VAR=$VALUE"
done <<< "$VARS"

# ── Step 9: set secrets ──────────────────────────────────────────────────────
info "Encoding config.toml and setting secrets..."
CONFIG_B64=$(base64 < "$CONFIG_PATH" | tr -d '\n')
# shellcheck disable=SC2086
$FLY secrets set CONFIG_TOML="$CONFIG_B64" $SECRET_ARGS --app "$APP_NAME"

# ── Step 10: substitute placeholders in fly.toml ─────────────────────────────
FLYTOML="$SCRIPT_DIR/fly.toml"
if grep -q 'DEXMON_APP_NAME' "$FLYTOML"; then
    info "Configuring fly/fly.toml..."
    if [ "$(uname)" = "Darwin" ]; then
        sed -i '' "s/DEXMON_APP_NAME/$APP_NAME/g;s/DEXMON_REGION/$REGION/g" "$FLYTOML"
    else
        sed -i "s/DEXMON_APP_NAME/$APP_NAME/g;s/DEXMON_REGION/$REGION/g" "$FLYTOML"
    fi
fi

# ── Step 11: deploy ──────────────────────────────────────────────────────────
info "Deploying to Fly.io..."
cd "$PROJECT_ROOT"
$FLY deploy --config fly/fly.toml

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
```

- [ ] **Step 2: Make script executable**

```bash
chmod +x fly/deploy.sh
```

- [ ] **Step 3: Lint with shellcheck (if available)**

```bash
shellcheck fly/deploy.sh
```

If shellcheck is not installed: `brew install shellcheck` (macOS) or `apt install shellcheck` (Linux). Linting is optional but recommended. Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add fly/deploy.sh
git commit -m "feat: add deploy.sh for first-time Fly.io setup"
```

---

### Task 4: update.sh

**Files:**
- Create: `fly/update.sh`

**Context:** `update.sh` reads the app name from `fly/fly.toml` directly (using `awk`) so there is no extra config file to maintain. If the app name is still the template placeholder `DEXMON_APP_NAME`, the script exits with a clear message directing the user to run `deploy.sh` first. Options 1, 2, and 4 all end with `fly deploy` because Fly.io requires a machine restart for secret changes to take effect.

- [ ] **Step 1: Create `fly/update.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

info() { printf '\033[0;32m==> \033[0m%s\n' "$*"; }
die()  { printf '\033[0;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

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
    cd "$PROJECT_ROOT"
    [ -f "$CONFIG_INPUT" ] || die "Config file not found: $CONFIG_INPUT"
    CONFIG_PATH="$(cd "$(dirname "$CONFIG_INPUT")" && pwd)/$(basename "$CONFIG_INPUT")"
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
    SECRET_ARGS=""
    while IFS= read -r VAR; do
        [ -z "$VAR" ] && continue
        printf 'Value for %s: ' "$VAR"
        read -rs VALUE
        echo ""
        SECRET_ARGS="$SECRET_ARGS $VAR=$VALUE"
    done <<< "$VARS"
    if [ -n "$SECRET_ARGS" ]; then
        # shellcheck disable=SC2086
        $FLY secrets set $SECRET_ARGS --app "$APP_NAME"
    fi
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
    1) get_config_path; push_config;             do_deploy ;;
    2) get_config_path;              push_secrets; do_deploy ;;
    3)                                             do_deploy ;;
    4) get_config_path; push_config; push_secrets; do_deploy ;;
    *) die "Invalid choice: $CHOICE" ;;
esac

info "Done."
```

- [ ] **Step 2: Make script executable**

```bash
chmod +x fly/update.sh
```

- [ ] **Step 3: Lint with shellcheck (if available)**

```bash
shellcheck fly/update.sh
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add fly/update.sh
git commit -m "feat: add update.sh for post-deploy config and secret changes"
```

---

### Task 5: GitHub Actions workflow

**Files:**
- Create: `.github/workflows/fly-deploy.yml`

**Context:** The workflow uses `--remote-only` so Fly builds the Docker image on their infrastructure — no Docker daemon is needed in the GitHub Actions runner. `concurrency: deploy-group` ensures only one deploy runs at a time if multiple pushes happen rapidly. The comment block at the top explains the workflow is optional and exactly what steps are needed to enable it. Users who don't want CI/CD simply delete this file or never add the `FLY_API_TOKEN` secret (the workflow will fail-fast without it, which is acceptable — it won't silently deploy nothing).

- [ ] **Step 1: Create `.github/workflows/fly-deploy.yml`**

```yaml
# Optional: Automatically deploy to Fly.io on every push to main.
#
# To enable:
#   1. Get a deploy token:   fly tokens create deploy -x 999999h
#   2. Add it to GitHub:     Settings → Secrets and variables → Actions
#                            → New repository secret
#                            Name:  FLY_API_TOKEN
#                            Value: <token from step 1>
#
# To disable: delete this file or remove the FLY_API_TOKEN secret.

name: Deploy to Fly.io

on:
  push:
    branches:
      - main

jobs:
  deploy:
    name: Deploy
    runs-on: ubuntu-latest
    concurrency: deploy-group
    steps:
      - uses: actions/checkout@v4

      - uses: superfly/flyctl-actions/setup-flyctl@master

      - name: Deploy
        run: flyctl deploy --remote-only --config fly/fly.toml
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/fly-deploy.yml
git commit -m "feat: add optional GitHub Actions workflow for Fly.io deployment"
```

---

### Task 6: README update

**Files:**
- Modify: `README.md` lines 276–281 (the existing "Free Cloud (Fly.io, Render, Railway)" stub)

**Context:** Replace the three-bullet stub with a full Fly.io deployment guide. Keep the section header focused on Fly.io only (removing the Render/Railway mentions since this is now Fly-specific). The callback URL note is critical — without it, emergency alarm acknowledgments won't work.

- [ ] **Step 1: Replace the existing Fly.io stub in README.md**

Find and replace this block (lines 276–281):

```markdown
### Free Cloud (Fly.io, Render, Railway)

- Set secrets via the provider's secret/env var mechanism
- Set `callback_url` to the provider's public URL
- Mount a persistent volume for the SQLite database — ephemeral storage will lose alarm state on restart
```

With:

```markdown
### Fly.io

dexmon ships with scripts for automated Fly.io deployment. The setup script guides first-time Fly.io users from zero to a running instance.

#### Prerequisites

Install `flyctl`:

```bash
curl -L https://fly.io/install.sh | sh
```

You'll also need a `config.toml` based on `config.toml.example` with your alarm rules filled in. Secrets (`${VAR}` values) are prompted for by the deploy script — do not put real credentials in the file.

#### Initial deploy

```bash
./fly/deploy.sh
```

The script:
1. Checks for `flyctl` and walks through login if needed
2. Prompts for an app name (globally unique on Fly.io) and region
3. Creates the Fly app and a 1 GB persistent volume for the SQLite database
4. Scans your `config.toml` for `${VAR}` references and prompts for each value
5. Uploads all secrets (including the encoded config) to Fly's encrypted secret store
6. Deploys the container

After the first deploy, update `callback_url` in your `config.toml`:

```toml
[server]
callback_url = "https://<your-app-name>.fly.dev/pushover/callback"
```

Then push the updated config:

```bash
./fly/update.sh   # choose option 1 or 4
```

#### Updating

```bash
./fly/update.sh
```

| Option | What it does |
|---|---|
| 1. Config file | Re-encode `config.toml`, push as secret, redeploy |
| 2. Secrets | Re-prompt for `${VAR}` values, push secrets, redeploy |
| 3. Code only | `fly deploy` — no secret changes |
| 4. Everything | Config + secrets + redeploy |

#### Optional: GitHub Actions CI/CD

To automatically deploy on every push to `main`:

1. Create a deploy token:

   ```bash
   fly tokens create deploy -x 999999h
   ```

2. Add it to your GitHub repo: **Settings → Secrets and variables → Actions → New repository secret**
   - Name: `FLY_API_TOKEN`
   - Value: the token from step 1

The workflow file is already at `.github/workflows/fly-deploy.yml`. To disable CI/CD, delete that file or remove the `FLY_API_TOKEN` secret.
```

- [ ] **Step 2: Verify the README renders correctly**

```bash
# Check the section looks right (no broken markdown)
grep -n "Fly.io\|fly/deploy\|fly/update\|callback_url\|FLY_API_TOKEN" README.md
```

Expected: each of those strings appears at least once in the output.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: replace Fly.io stub with full deployment guide"
```

---

## Self-Review Against Spec

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Two-stage Dockerfile (Go builder → Alpine runtime) | Task 1 |
| `ca-certificates` in runtime image | Task 1 |
| `entrypoint.sh` decodes `CONFIG_TOML` to `/data/config.toml` | Task 1 |
| `exec` dexmon with `-config` and `-db` flags | Task 1 |
| `fly.toml`: port 8080, force_https, auto_stop_machines=false, min_machines_running=1 | Task 2 |
| `fly.toml`: persistent volume at `/data` | Task 2 |
| `deploy.sh`: check flyctl, walk through auth | Task 3 |
| `deploy.sh`: prompt app name and region with defaults | Task 3 |
| `deploy.sh`: create app (skip if exists) | Task 3 |
| `deploy.sh`: create volume (skip if exists) | Task 3 |
| `deploy.sh`: parse `${VAR}` from config.toml, prompt for values | Task 3 |
| `deploy.sh`: set all secrets in one call including `CONFIG_TOML` | Task 3 |
| `deploy.sh`: substitute placeholders in fly.toml (macOS/Linux portable) | Task 3 |
| `deploy.sh`: run `fly deploy`, print callback_url reminder | Task 3 |
| `update.sh`: detect app name from fly.toml | Task 4 |
| `update.sh`: 4-option menu (config / secrets / code / all) | Task 4 |
| `update.sh`: all options ending with redeploy call `fly deploy` | Task 4 |
| GitHub Actions: push to main → fly deploy --remote-only | Task 5 |
| GitHub Actions: optional, comment block explaining enable/disable | Task 5 |
| GitHub Actions: concurrency group to prevent parallel deploys | Task 5 |
| README: replace stub with full guide | Task 6 |
| README: callback_url setup instructions | Task 6 |
| README: CI/CD enable instructions | Task 6 |

**All spec sections covered. No placeholders in code. Types and function signatures consistent across tasks.**
