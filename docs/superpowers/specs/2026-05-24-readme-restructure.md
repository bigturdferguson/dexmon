# README Restructure Design

**Date:** 2026-05-24
**Status:** Approved

## Overview

Restructure `README.md` so that Fly.io is the clear, default deployment path. A new reader lands on the README and has everything they need to deploy without clicking away. The Raspberry Pi and local-run paths are demoted to linked guides. A dashboard section is added to showcase the web UI.

---

## Goals

- First-time user lands on README and can deploy to Fly.io without leaving the page
- Secrets configuration is explained clearly enough for a non-technical user to succeed
- Dashboard is discoverable and its URL format is explicit
- Local/Pi paths are still accessible but not in the main flow

---

## File Changes

| File | Action | Purpose |
|------|--------|---------|
| `README.md` | Rewrite | New structure: intro → dashboard → Fly.io deploy → config → updating → other methods |
| `guides/local-run.md` | Create | Absorb old Quick Start + env var content; "testing only" warning at top |
| `guides/fly-io.md` | Keep (trim) | Retain troubleshooting section; remove setup steps that now live in README |
| `guides/github-actions.md` | Keep (unchanged) | Referenced from Fly.io deploy section |
| `guides/raspberry-pi.md` | Keep (unchanged) | Referenced from "other methods" section |

---

## README Structure

### 1. Header + Intro

One tight paragraph describing what dexmon does and who it's for. Fold the current "How It Works" prose into this paragraph — no separate heading needed.

### 2. Dashboard

Own `## Dashboard` section. Content:

- One sentence describing the dashboard
- Widget list:
  - Current BG (value, trend arrow, time since reading)
  - Previous reading
  - 24-hour high, low, and average
  - BG graph (last 24 hours)
  - Alarm summary (name, priority, last fired, status)
- URL: `https://<appname>.fly.dev/`
- Auto-refreshes every 5 minutes; light/dark theme toggle

### 3. Deploy to Fly.io

`## Deploy to Fly.io` — the full first-time setup flow, inline. This replaces the current Quick Start section.

**Subsections:**

#### Prerequisites
- flyctl installed (one-liner curl command)
- Fly.io account (free, link to fly.io)
- Pushover account + application created
- Dexcom Share enabled on the patient's app

#### 1. Get your credentials

Three credential groups, each with step-by-step instructions:

**Pushover app token** — pushover.net → Your Applications → click app → copy API Token/Key

**Pushover user key** — pushover.net → User Key shown at top of page after login. One per person receiving notifications.

**Dexcom credentials** — the email and password used to log in to the Dexcom mobile app.

#### 2. Prepare config.toml

```bash
cp config.toml.example config.toml
```

Explain the `${VAR}` pattern explicitly:

> `config.toml` uses `${VARIABLE_NAME}` placeholders for all credentials — the actual values are never written into the file. Instead, the deploy script reads these placeholders, prompts you for the real values, and uploads them directly to Fly's encrypted secret store. You type the value once; it never touches your disk.

Tell the reader to fill in alarm rules and recipients, and leave `callback_url = ""` for now.

#### 3. Run `./fly/deploy.sh`

Explain what happens at each prompt:

- **App name** — globally unique across all Fly.io users. `dexmon` is taken — use `dexmon-noah` or `dexmon-yourname`. Becomes the URL: `https://<appname>.fly.dev`.
- **Region** — closest data center. Provide the same region table as current fly-io.md.
- **Secret prompts** — this is the most important part for non-technical users. Explain clearly:
  - The script scans `config.toml` for every `${VAR}` reference
  - It prompts for each one by name — e.g., `Value for PUSHOVER_APP_TOKEN:`
  - Input is hidden (nothing appears as you type — this is normal)
  - Paste or type the credential value and press Enter
  - Values go directly to Fly's encrypted store; they are never saved locally

After prompts, the script: creates the app, creates a 1 GB volume for SQLite, uploads secrets, builds and deploys remotely.

#### 4. Set the callback URL

After first deploy, update `config.toml`:

```toml
callback_url = "https://<appname>.fly.dev/pushover/callback"
```

Then run `./fly/update.sh` → option 1.

#### 5. Verify

```bash
fly logs --app <appname>
```

Show healthy log output. Explain what "no alarm" means.

#### Automate deploys with CI/CD

One paragraph + link to `guides/github-actions.md`.

### 4. Configuration

Retain current `## Configuration` section unchanged — it's comprehensive and correct.

### 5. Updating

`## Updating` section with the `./fly/update.sh` options table (currently in fly-io.md, move here):

| Option | When to use |
|--------|-------------|
| 1. Config file | Changed alarm rules, thresholds, or callback_url |
| 2. Secrets | Rotated a Dexcom or Pushover credential |
| 3. Code only | Pulled new code from git, no config or secret changes |
| 4. Everything | Changed config and rotated secrets at the same time |

### 6. Other Deployment Methods

`## Other Deployment Methods` — short section, two bullets:

- **Local / test run** — link to `guides/local-run.md`. Note: no persistent storage, no public callback URL — not suitable for production.
- **Raspberry Pi** — link to `guides/raspberry-pi.md`. Always-on self-hosted option.

Remove the current Deployment table and the Flags and Error Handling sections from the README (this detail belongs in guides, not the main README).

---

## New File: `guides/local-run.md`

**Warning at top:** This is for local testing only. There is no persistent storage (database is lost on restart) and no public URL for emergency alarm callbacks.

**Content (from current README):**
- Build: `go build -o dexmon .`
- Environment variables table (currently in README)
- Run: `./dexmon -config config.toml -db dexmon.db`
- Flags table

---

## Trimmed `guides/fly-io.md`

Remove the setup steps (Prerequisites through "Set the callback URL") since they now live in the README. Retain:
- Troubleshooting section (unchanged)
- Any content not duplicated in README

Retitle the file or add a note at the top linking back to the README for setup.

---

## Tone and Clarity Notes

- The secrets prompt section must explain that hidden input is normal — non-technical users panic when nothing appears as they type
- Avoid jargon: say "Dexcom mobile app" not "Dexcom Share client"; say "copy the token" not "extract the API key"
- The `${VAR}` pattern needs a plain-English explanation on first encounter — it is confusing to non-developers
- Credential prompts: explicitly name which credential maps to which prompt (e.g., "when the script asks for `PUSHOVER_APP_TOKEN`, paste the API Token/Key you copied from pushover.net")
