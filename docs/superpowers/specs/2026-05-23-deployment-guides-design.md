# Deployment Guides Design

**Date:** 2026-05-23
**Status:** Approved

## Overview

Add three step-by-step deployment guides in a `guides/` directory (committed to git, visible on GitHub). Improve the README to be a scannable reference doc that links to the guides for deployment detail. No `guides/README.md` — the main README remains the single conceptual home.

---

## File Structure

```
guides/
  raspberry-pi.md      # full Pi deployment walkthrough
  fly-io.md            # full Fly.io deployment walkthrough
  github-actions.md    # CI/CD setup guide
README.md              # updated: adds "how it works", accounts vs recipients, shrinks Deployment section to links
```

---

## Audience

Anyone who finds the repo. Assume no prior knowledge of Fly.io, systemd, or GitHub Actions. Assume the reader has Pushover and Dexcom accounts already — explain where to find tokens/keys, do not reproduce Pushover or Dexcom's own onboarding docs.

---

## README Changes

Three targeted changes, everything else stays:

1. **Add "How it works" section** near the top — 3-4 sentences covering the poll loop, alarm evaluation (threshold + trend filter), Pushover notification delivery, and callback acknowledgment for emergency alarms.

2. **Add accounts vs. recipients explanation** — one short paragraph explaining why Dexcom accounts (what to monitor) and recipients (who to notify) are separate, and that multiple recipients can watch the same account with independent snooze/backoff state.

3. **Replace the Deployment section** with a short table:

   | Platform | Guide |
   |---|---|
   | Raspberry Pi (home, always-on) | [guides/raspberry-pi.md](guides/raspberry-pi.md) |
   | Fly.io (cloud, no hardware) | [guides/fly-io.md](guides/fly-io.md) |
   | GitHub Actions CI/CD | [guides/github-actions.md](guides/github-actions.md) |

   One sentence of context for each link.

---

## guides/raspberry-pi.md

**Target reader:** Someone with a Pi running Raspberry Pi OS Lite 64-bit (Bookworm) and SSH access, who wants to run dexmon as a persistent background service.

### Sections

1. **Prerequisites**
   - Raspberry Pi running Raspberry Pi OS Lite 64-bit (Bookworm), SSH access confirmed
   - Pushover account with an application created (free)
   - Dexcom Share enabled on the patient's Dexcom account

2. **Get your credentials**
   - Pushover app token: pushover.net → Your Apps → select app → API Token/Key
   - Pushover user key: pushover.net → Your Profile → User Key
   - Dexcom Share username (email) and password

3. **Install Go on the Pi**
   - Download the official ARM64 tarball from go.dev/dl
   - Extract to `/usr/local`, add to PATH in `~/.profile`
   - Verify with `go version`

4. **Get dexmon**
   - `git clone` the repo onto the Pi (or copy files via scp)
   - `go build -o dexmon .`

5. **Configure**
   - `cp config.toml.example config.toml`, edit to match alarms and recipients
   - Create `/opt/dexmon/secrets.env` with all `${VAR}` values, mode 600, owned by root
   - The systemd unit file references this secrets file via `EnvironmentFile`

6. **Install as a systemd service**
   - Copy binary and config to `/opt/dexmon/`
   - Copy `dexmon.service` to `/etc/systemd/system/`
   - `sudo systemctl daemon-reload && sudo systemctl enable --now dexmon`
   - Verify: `journalctl -u dexmon -f`

7. **Enable emergency callbacks via Cloudflare Tunnel**
   - Install `cloudflared` (one-line install from Cloudflare's repo)
   - `cloudflared tunnel --url http://localhost:8080` to get a public HTTPS URL
   - Update `callback_url` in `config.toml` with the tunnel URL
   - Run cloudflared as a systemd service so it survives reboots
   - Restart dexmon

8. **Verify**
   - What healthy log output looks like (reading received, no errors)
   - How to confirm a Pushover notification was delivered

9. **Troubleshooting**
   - Wrong Dexcom credentials → "session expired" log loop, fix secrets.env and restart
   - Pushover token wrong → dispatch errors in logs, verify token vs user key distinction
   - Tunnel not running → callbacks fail silently, check cloudflared service status

---

## guides/fly-io.md

**Target reader:** Someone who wants dexmon running in the cloud without any local hardware. Assumes no prior Fly.io experience.

### Sections

1. **Prerequisites**
   - `flyctl` installed (`curl -L https://fly.io/install.sh | sh`)
   - Fly.io account created at fly.io (free tier works)
   - Pushover app created, Dexcom Share enabled

2. **Get your credentials**
   - Same as Pi guide: Pushover app token, user key, Dexcom Share login

3. **Prepare your config.toml**
   - Copy `config.toml.example`, fill in alarm rules and recipients
   - Leave all `${VAR}` placeholders exactly as-is — the deploy script prompts for the values
   - Leave `callback_url` as the placeholder for now (you'll get the real URL after first deploy)

4. **Run `./fly/deploy.sh`**
   - Detailed walkthrough of every prompt:
     - App name: globally unique across all Fly.io users, `dexmon` is likely taken
     - Region: closest to you for lowest latency — link to Fly regions list
     - Each secret prompt: what it is and where to find it
   - What each step does behind the scenes (app creation, volume, secrets upload, deploy)

5. **Set the callback URL**
   - After deploy you have `https://<appname>.fly.dev`
   - Update `callback_url = "https://<appname>.fly.dev/pushover/callback"` in `config.toml`
   - Run `./fly/update.sh` and choose option 1 (config file)

6. **Verify**
   - `fly logs --app <appname>` — what healthy output looks like
   - Confirm a reading was received in the logs

7. **Updating**
   - `./fly/update.sh` menu explained in detail:
     - Option 1: changed alarm rules or callback URL → re-encode config
     - Option 2: rotated a Dexcom/Pushover credential → re-enter secrets
     - Option 3: pulled new code from git → code-only redeploy
     - Option 4: changed config AND rotated secrets simultaneously

8. **Troubleshooting**
   - Container won't start: `CONFIG_TOML` missing or malformed — check `fly secrets list`
   - Dexcom auth failures: wrong username/password in secrets — use `update.sh` option 2
   - Missing secrets: a `${VAR}` in config.toml has no matching secret set

---

## guides/github-actions.md

**Target reader:** Someone who has completed the Fly.io guide and wants pushes to `main` to automatically deploy.

### Sections

1. **Prerequisites**
   - Fly.io guide completed and dexmon is running
   - `fly/fly.toml` has your real app name (not the `DEXMON_APP_NAME` placeholder) committed to the repo
   - Repo is hosted on GitHub

2. **Create a deploy token**
   - `fly tokens create deploy -x 999999h`
   - What this token does: allows deploying to your app without full account access
   - Why it's separate from your account password: limited scope, safe to store in GitHub

3. **Add to GitHub**
   - Repository → Settings → Secrets and variables → Actions → New repository secret
   - Name: `FLY_API_TOKEN`, Value: token from previous step

4. **How it works**
   - Workflow triggers on push to `main`
   - Checks out code, installs flyctl, runs `flyctl deploy --remote-only --config fly/fly.toml`
   - `--remote-only`: Fly builds the Docker image on their infrastructure — no Docker daemon needed in CI
   - `concurrency: deploy-group`: only one deploy runs at a time if pushes stack up

5. **Trigger your first deploy**
   - Push any commit to `main`
   - Watch: GitHub → Actions tab → Deploy to Fly.io workflow

6. **Verify**
   - Green checkmark in Actions tab
   - `fly logs --app <appname>` shows the new version starting up

7. **Disabling CI/CD**
   - Remove `FLY_API_TOKEN` from GitHub secrets to disable without deleting the workflow file (easy to re-enable)
   - Or delete `.github/workflows/fly-deploy.yml` to remove it entirely

8. **Troubleshooting**
   - Workflow fails immediately with auth error: `FLY_API_TOKEN` not set or wrong value
   - Deploy succeeds but app name not found: `fly/fly.toml` still has `DEXMON_APP_NAME` placeholder — run `deploy.sh` locally first and commit the result
   - Deploy succeeds in CI but app doesn't start: check `fly logs` for startup errors (usually a missing secret)
