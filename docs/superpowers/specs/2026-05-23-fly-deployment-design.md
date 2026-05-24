# dexmon Fly.io Deployment Design

**Date:** 2026-05-23
**Status:** Approved

## Overview

Add automated Fly.io deployment for dexmon via two bash scripts (`deploy.sh` for first-time setup, `update.sh` for subsequent changes) plus an optional GitHub Actions workflow for continuous deployment on push to `main`. The deployment targets first-time Fly.io users — the scripts check prerequisites and guide through authentication before doing anything.

---

## File Structure

```
Dockerfile
fly/
  entrypoint.sh       # decode CONFIG_TOML secret, exec dexmon
  fly.toml            # Fly app config template (app name substituted by deploy.sh)
  deploy.sh           # first-time setup: create app, volume, secrets, deploy
  update.sh           # post-deploy: push config/secret/code changes
.github/
  workflows/
    fly-deploy.yml    # optional CI/CD: push to main → fly deploy
```

The README's existing Fly.io placeholder section is replaced with a full deployment guide.

---

## Container Design

### Dockerfile

Two-stage build:
- **Stage 1** (`golang` alpine image matching the version in `go.mod`): compile the dexmon binary
- **Stage 2** (`alpine:latest`): copy binary + `entrypoint.sh`, set entrypoint

Alpine is required (not scratch) because `entrypoint.sh` needs `/bin/sh` and the `base64` command.

### entrypoint.sh

Runs as the container entrypoint on every start:

1. Validates `CONFIG_TOML` env var is set; exits with error if missing
2. Decodes `CONFIG_TOML` (base64) to `/data/config.toml`
3. `exec`s dexmon: `dexmon -config /data/config.toml -db /data/dexmon.db`

The config is decoded fresh on every container start. Secret changes (including config changes) take effect on the next `fly deploy` or `fly machine restart` without touching the volume.

### fly.toml

- `primary_region` set by `deploy.sh` at first deploy
- `app` name set by `deploy.sh` via substitution
- `[[services]]`: exposes port 8080 as an HTTPS service (Fly terminates TLS). External URL is `https://<appname>.fly.dev`.
- `[[mounts]]`: persistent volume `dexmon_data` mounted at `/data`

Fly handles TLS termination, so `callback_url` in the user's config.toml is simply `https://<appname>.fly.dev/pushover/callback` — no certificate management required.

---

## Secrets Strategy

All secrets are stored in Fly's encrypted secret store and injected as environment variables at runtime.

| Secret | Source | Description |
|---|---|---|
| `CONFIG_TOML` | base64(config.toml) | Full config file, decoded to disk by entrypoint.sh |
| `PUSHOVER_APP_TOKEN` | user prompt | Pushover application token |
| `PUSHOVER_USER_KEY_*` | parsed from config.toml `${...}` refs | Per-recipient Pushover keys |
| `DEXCOM_USER_*` | parsed from config.toml `${...}` refs | Dexcom username per account |
| `DEXCOM_PASS_*` | parsed from config.toml `${...}` refs | Dexcom password per account |
| `HEALTHCHECKS_PING_URL` | parsed from config.toml `${...}` refs | Watchdog ping URL (if configured) |

`deploy.sh` parses the user's config.toml for all `${VAR_NAME}` references and prompts for each one, so the secret list is automatically derived from the config rather than hardcoded. All secrets are set in a single `fly secrets set` call.

---

## deploy.sh

Guides a first-time user from zero to running. Steps:

1. **Check flyctl** — if not found on PATH, print install URL (`https://fly.io/install.sh`) and exit with a clear error
2. **Check auth** — run `fly auth whoami`; if unauthenticated, run `fly auth login` and wait for completion
3. **App name** — prompt with default `dexmon` (globally unique on Fly.io)
4. **Region** — prompt with default `iad`; print link to Fly regions list
5. **Create app** — `fly apps create <name>`; skip gracefully if app already exists
6. **Create volume** — `fly volumes create dexmon_data --size 1 --region <region> --app <name>`; skip if volume already exists
7. **Config file** — prompt for path (default: `./config.toml`); validate file exists
8. **Discover secrets** — parse config.toml for all `${VAR_NAME}` references; prompt for the value of each one
9. **Set secrets** — encode config.toml as base64, set `CONFIG_TOML` plus all discovered env vars in one `fly secrets set` call
10. **Substitute app name** — write `fly/fly.toml` with the chosen app name
11. **Deploy** — `fly deploy`
12. **Post-deploy instructions** — print the public URL (`https://<appname>.fly.dev`); remind user to set `callback_url = "https://<appname>.fly.dev/pushover/callback"` in config.toml and run `update.sh` to push the updated config

---

## update.sh

Used after the initial deploy. Reads app name from `fly/fly.toml`. Checks flyctl and auth, then presents a menu:

```
What would you like to update?
1. Config file (re-encode config.toml and push as CONFIG_TOML secret)
2. Secrets (re-prompt for ${VAR} values found in config.toml)
3. Code only (fly deploy)
4. Everything (config + secrets + deploy)
```

Options 1, 2, and 4 end with `fly deploy` because Fly requires a restart for secret changes to take effect.

---

## GitHub Actions (optional)

`.github/workflows/fly-deploy.yml`:

- **Trigger:** push to `main`
- **Action:** `superfly/flyctl-actions/setup-flyctl@master`, then `fly deploy --remote-only`
- **`--remote-only`:** Fly builds the image on their infrastructure; no local Docker daemon needed in CI
- **Required GitHub secret:** `FLY_API_TOKEN` (obtained via `fly tokens create deploy -x 999999h`)

The workflow file has a comment block at the top explaining it is optional. Users who do not want CI/CD can delete the file. The README deployment section includes a subsection explaining how to enable it (add `FLY_API_TOKEN` to GitHub repository secrets).

---

## README Changes

Replace the existing one-paragraph Fly.io stub in the Deployment section with:

- **Prerequisites** — link to flyctl install instructions
- **Initial deploy** — `./fly/deploy.sh` with explanation of what it does
- **Updating** — `./fly/update.sh` with menu description
- **Callback URL note** — reminder to set `callback_url` after first deploy and push via update.sh
- **Optional CI/CD** — how to add `FLY_API_TOKEN` to GitHub and what the workflow does

---

## Error Handling

| Failure | Behavior |
|---|---|
| `flyctl` not found | Print install URL, exit 1 |
| Not authenticated | Run `fly auth login` interactively |
| App name already taken by another user | Fly returns error; script prints message and re-prompts |
| config.toml not found at given path | Print error, exit 1 |
| No `${VAR}` references found in config.toml | Warn user; still set `CONFIG_TOML` and continue |
| `fly deploy` fails | Show flyctl output; user re-runs after fixing |
| Volume already exists | Detected via `fly volumes list`; skip creation silently |
