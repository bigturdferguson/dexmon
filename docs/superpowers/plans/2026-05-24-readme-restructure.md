# README Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the README so Fly.io is the default deployment path, add a dashboard section, demote local run to a testing-only guide, and trim guides/fly-io.md to troubleshooting only.

**Architecture:** Pure documentation changes — three files modified, one file created. No Go code changes. No tests involved. Verification is manual link-checking and content review.

**Tech Stack:** Markdown.

---

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `README.md` | Rewrite | New structure: intro → dashboard → Fly.io deploy (full inline) → config → updating → other methods |
| `guides/local-run.md` | Create | Testing-only warning + build/run steps + env var table + flags |
| `guides/fly-io.md` | Modify | Remove setup steps (now in README); keep troubleshooting; add link back to README |
| `guides/github-actions.md` | Unchanged | No modifications needed |
| `guides/raspberry-pi.md` | Unchanged | No modifications needed |

---

## Task 1: Create `guides/local-run.md`

**Files:**
- Create: `guides/local-run.md`

- [ ] **Step 1.1: Create the file**

Use the Write tool to create `guides/local-run.md` with this exact content:

```markdown
# Running dexmon Locally (Testing Only)

> **This is for local testing only.** There is no persistent storage — the database is lost when the process stops — and there is no public URL for emergency alarm callbacks. For production use, [deploy to Fly.io](../README.md#deploy-to-flyio).

---

## Build

```bash
go build -o dexmon .
```

The binary has no runtime dependencies.

---

## Set environment variables

Set one variable per credential. The names must match the `${VAR}` placeholders in your `config.toml` exactly.

```bash
export PUSHOVER_APP_TOKEN=your_app_token_here
export PUSHOVER_USER_KEY_BRANDON=your_user_key_here   # one per recipient
export DEXCOM_USER_NOAH=noah@example.com
export DEXCOM_PASS_NOAH=dexcom_password_here
export HEALTHCHECKS_PING_URL=https://hc-ping.com/...  # optional
```

| Variable | Required | Description |
|---|---|---|
| `PUSHOVER_APP_TOKEN` | Yes | Pushover application token — pushover.net → Your Applications → API Token/Key |
| `PUSHOVER_USER_KEY_<NAME>` | Per recipient | Pushover user key — pushover.net → User Key (top of page after login) |
| `DEXCOM_USER_<NAME>` | Per account | Dexcom login email |
| `DEXCOM_PASS_<NAME>` | Per account | Dexcom login password |
| `HEALTHCHECKS_PING_URL` | No | Watchdog ping URL; leave unset to disable |

The `<NAME>` suffix must match the placeholder in `config.toml`. If your config has `${DEXCOM_USER_NOAH}`, set `DEXCOM_USER_NOAH`.

---

## Run

```bash
./dexmon -config config.toml -db dexmon.db
```

| Flag | Default | Description |
|---|---|---|
| `-config` | `config.toml` | Path to the TOML config file |
| `-db` | `dexmon.db` | Path to the SQLite database file |

The process logs to stdout. Press Ctrl+C to stop. The database file (`dexmon.db`) is created in the current directory on first run.
```

- [ ] **Step 1.2: Verify the file exists**

```bash
test -f guides/local-run.md && echo "OK" || echo "MISSING"
```

Expected: `OK`

- [ ] **Step 1.3: Commit**

```bash
git add guides/local-run.md
git commit -m "docs: add local-run testing guide"
```

---

## Task 2: Rewrite `README.md`

**Files:**
- Modify: `README.md`

- [ ] **Step 2.1: Read the current README**

Read `README.md` in full before making any changes. You need the complete `## Configuration` and `## Alarm Priority Reference` sections to copy verbatim into the new README.

- [ ] **Step 2.2: Write the new README**

Use the Write tool to replace `README.md` with this exact content (the Configuration and Alarm Priority Reference sections are carried over unchanged from the current file):

```markdown
# dexmon

A long-running Go daemon that monitors a Dexcom Share CGM account, evaluates configurable alarm rules, and sends Pushover notifications. Every poll interval it fetches the latest reading; when blood glucose crosses a threshold and the current trend matches the configured filter, a notification is dispatched. Emergency alarms retry until acknowledged — when a recipient acknowledges from the Pushover app, dexmon's webhook stops retrying and can optionally snooze further alerts for that recipient.

---

## Dashboard

The dashboard is available at `https://<appname>.fly.dev/` and auto-refreshes every 5 minutes.

| Widget | Shows |
|--------|-------|
| Current BG | Value, trend arrow (↑↑ ↑ ↗ → ↘ ↓ ↓↓), and time since reading |
| Previous | Prior reading value and age |
| High | Maximum BG over the last 24 hours |
| Low | Minimum BG over the last 24 hours |
| Avg | Integer average BG over the last 24 hours |
| BG Graph | 24-hour line chart with a shaded 70–180 target range |
| Alarms | Per-alarm name, priority, last fired time, and current status |

Supports light and dark themes — toggle with the button in the header. Preference is saved across page loads.

---

## Deploy to Fly.io

Fly.io is the recommended way to run dexmon. You get a persistent database volume, automatic HTTPS (required for emergency alarm callbacks), and a machine that runs 24/7. The free tier is sufficient.

### Prerequisites

- **flyctl** installed:
  ```bash
  curl -L https://fly.io/install.sh | sh
  ```
- A **Fly.io account** at [fly.io](https://fly.io) — free, no credit card required
- A **Pushover** account at [pushover.net](https://pushover.net) with an application created
- **Dexcom Share** enabled on the patient's Dexcom G-series app (Settings → Share → Invite Followers)

### 1. Gather your credentials

You need three things before running the deploy script.

**Pushover app token**

Go to [pushover.net](https://pushover.net) → scroll to **Your Applications** → click your app → copy the **API Token/Key**. This is your `PUSHOVER_APP_TOKEN`.

**Pushover user key**

On [pushover.net](https://pushover.net), your **User Key** is shown at the top of the page after logging in. Each person who receives notifications needs their own user key — this is `PUSHOVER_USER_KEY_<NAME>`.

**Dexcom credentials**

The email address and password used to log in to the Dexcom mobile app. These are `DEXCOM_USER_<NAME>` and `DEXCOM_PASS_<NAME>`.

### 2. Prepare config.toml

```bash
cp config.toml.example config.toml
```

Open `config.toml` and fill in your alarm rules, thresholds, and recipient names. Leave all `${VARIABLE_NAME}` placeholders exactly as they are — **do not replace them with real credentials**.

> **How `${VAR}` placeholders work:** Every credential in `config.toml` is written as `${VARIABLE_NAME}` — a placeholder, never the real value. The deploy script scans the file, finds each placeholder, and prompts you for the actual value. Your credentials go directly into Fly's encrypted secret store and never touch your disk. You type each value once at the prompt; that's it.

Leave `callback_url = ""` for now — you will fill this in after the first deploy.

### 3. Run `./fly/deploy.sh`

```bash
./fly/deploy.sh
```

The script walks you through three prompts before deploying.

**App name**

Choose a name that is unique across all Fly.io users — `dexmon` is already taken. Use something like `dexmon-noah` or `dexmon-yourname`. This becomes your URL: `https://<appname>.fly.dev`.

**Region**

The Fly.io data center closest to you:

| Code | Location |
|------|----------|
| `iad` | Northern Virginia (US East) |
| `ord` | Chicago (US Central) |
| `lax` | Los Angeles (US West) |
| `lhr` | London (Europe) |
| `syd` | Sydney (Australia) |

Full list: [fly.io/docs/reference/regions](https://fly.io/docs/reference/regions/)

**Secret prompts**

The script finds every `${VAR}` placeholder in your `config.toml` and asks for its value one at a time:

```
Value for PUSHOVER_APP_TOKEN (NEW — required):
```

A few things to know:
- **Nothing appears as you type — this is normal.** Input is hidden to protect your credentials.
- Paste or type the value and press Enter.
- Each prompt name tells you exactly which credential is needed:

| Prompt | What to enter |
|--------|--------------|
| `PUSHOVER_APP_TOKEN` | API Token/Key — pushover.net → Your Applications → click your app |
| `PUSHOVER_USER_KEY_<NAME>` | User Key — pushover.net, shown at top of page after login |
| `DEXCOM_USER_<NAME>` | Dexcom login email |
| `DEXCOM_PASS_<NAME>` | Dexcom login password |
| `HEALTHCHECKS_PING_URL` | Ping URL from healthchecks.io — leave blank if not using |

After all prompts, the script creates the Fly app, provisions a 1 GB persistent volume for the database, uploads your secrets, and builds and deploys the container remotely.

### 4. Set the callback URL

After the first deploy your app is live at `https://<appname>.fly.dev`. Open `config.toml` and update:

```toml
[server]
callback_url = "https://<appname>.fly.dev/pushover/callback"
```

Then push the updated config:

```bash
./fly/update.sh
```

Choose **option 1** (Config file). The script re-encodes `config.toml` and redeploys.

### 5. Verify

```bash
fly logs --app <appname>
```

Healthy output looks like:

```
[noah] reading: 142 → (no alarm)
[noah] reading: 138 → (no alarm)
```

A new line appears every poll interval. The dashboard is live at `https://<appname>.fly.dev/`.

### Automate deploys with CI/CD

To automatically redeploy on every push to `main`, see [guides/github-actions.md](guides/github-actions.md).

---

## Configuration

All configuration lives in a single TOML file. Secrets are referenced as `${ENV_VAR_NAME}` and are expanded from the environment at startup — the literal `${...}` tokens are never written to the config file.

**Accounts** are the Dexcom Share logins dexmon monitors — one entry per patient. **Recipients** are the people who receive Pushover notifications. They are separate: multiple recipients can watch the same account, and each has independent snooze and backoff state. A recipient who silences an alarm does not affect what others receive.

### `[server]`

```toml
[server]
callback_port = 8080
callback_url  = "https://your-domain.com/pushover/callback"
```

| Key | Description |
|---|---|
| `callback_port` | Local port the server listens on. Serves the web dashboard at `/` and the Pushover webhook at `/pushover/callback`. |
| `callback_url` | Public URL Pushover uses to deliver acknowledgments. Must be reachable from the internet for emergency alarms to support acknowledgment/snooze. |

### `[health]`

```toml
[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = ["brandon"]

  [health.watchdog]
  ping_url = "${HEALTHCHECKS_PING_URL}"
```

**`[health.dexcom_timeout]`** — fires a Pushover alert when polling fails `max_missed_readings` times in a row for an account.

| Key | Description |
|---|---|
| `max_missed_readings` | Number of consecutive failed polls before alerting. Must be > 0 when recipients are configured. |
| `priority` | Pushover priority: `normal`, `high`, or `emergency` |
| `recipients` | List of recipient names to notify (must exist in `[recipients]`) |

**`[health.watchdog]`** — dead man's switch. dexmon pings `ping_url` after every successful poll. If pings stop (process crashed, host down), the external service sends its own alert.

| Key | Description |
|---|---|
| `ping_url` | URL to GET on each successful poll. Works with healthchecks.io (free tier), BetterUptime, etc. Leave empty to disable. |

### `[recipients]`

Define one entry per person who can receive notifications.

```toml
[recipients]
  [recipients.brandon]
  pushover_user_key = "${PUSHOVER_USER_KEY_BRANDON}"

  [recipients.sarah]
  pushover_user_key = "${PUSHOVER_USER_KEY_SARAH}"
```

| Key | Description |
|---|---|
| `pushover_user_key` | Pushover user key for this recipient (from pushover.net → Your Profile) |

Recipient names are referenced by `recipients = [...]` in alarms. The name is arbitrary — it just needs to match consistently.

### `[accounts]`

Define one entry per Dexcom Share account to monitor.

```toml
[accounts]
  [accounts.jessica]
  dexcom_username = "${DEXCOM_USER_JESSICA}"
  dexcom_password = "${DEXCOM_PASS_JESSICA}"
  poll_interval   = "5m"
```

| Key | Description |
|---|---|
| `dexcom_username` | Dexcom Share login email |
| `dexcom_password` | Dexcom Share login password |
| `poll_interval` | How often to poll for a new reading. Go duration string: `5m`, `3m`, etc. Dexcom updates every 5 minutes. |

### `[[accounts.<name>.alarms]]`

Each account can have multiple alarms. Each alarm is a `[[accounts.<name>.alarms]]` table.

```toml
[[accounts.jessica.alarms]]
name              = "Severe Low"
threshold         = 55
direction         = "below"
trend             = ["double_up", "single_up", "forty_five_up", "flat",
                     "forty_five_down", "single_down", "double_down",
                     "not_computable", "rate_out_of_range", "none"]
priority          = "emergency"
retry             = "5m"
expire            = "2h"
rearm_on_recovery = true
recipients        = ["brandon", "sarah", "jessica"]
```

| Key | Type | Description |
|---|---|---|
| `name` | string | Human-readable alarm name (appears in notifications) |
| `threshold` | integer | Blood glucose value in mg/dL |
| `direction` | `above` / `below` | Alarm fires when BG is above or below the threshold |
| `trend` | list of trends | Alarm fires only when the current trend is in this list. Use all trends to fire regardless of direction. |
| `priority` | `normal` / `high` / `emergency` | Pushover priority. Emergency bypasses quiet hours and retries until acknowledged. |
| `retry` | duration | **Emergency only.** How often Pushover re-notifies if unacknowledged (e.g. `"5m"`). |
| `expire` | duration | **Emergency only.** How long Pushover keeps retrying (e.g. `"2h"`). |
| `backoff` | duration | **Non-emergency.** Minimum time between repeat fires for the same alarm per recipient. Omit to fire every poll. |
| `rearm_on_recovery` | bool | If `true`, backoff and snooze reset when BG recovers past the threshold. If `false`, original backoff is honored across recoveries. |
| `recipients` | list of names | Which recipients to notify. Each gets an individual notification; one person's snooze does not affect others. |

#### Trend values

| Value | Arrow | Meaning |
|---|---|---|
| `double_up` | ↑↑ | Rising rapidly |
| `single_up` | ↑ | Rising |
| `forty_five_up` | ↗ | Rising slightly |
| `flat` | → | Stable |
| `forty_five_down` | ↘ | Falling slightly |
| `single_down` | ↓ | Falling |
| `double_down` | ↓↓ | Falling rapidly |
| `not_computable` | ? | Sensor cannot compute trend |
| `rate_out_of_range` | — | Rate too extreme to classify |
| `none` | | No trend data |

---

## Alarm Priority Reference

| Config priority | Behavior |
|---|---|
| `normal` | Standard Pushover notification, respects quiet hours |
| `high` | Bypasses Pushover quiet hours |
| `emergency` | Retries every `retry` until acknowledged or `expire` elapsed; supports callback acknowledgment and snooze |

---

## Updating

Run `./fly/update.sh` whenever you need to make changes:

| Option | When to use |
|--------|-------------|
| **1. Config file** | Changed alarm rules, thresholds, or `callback_url` |
| **2. Secrets** | Rotated a Dexcom or Pushover credential |
| **3. Code only** | Pulled new code from git, no config or secret changes |
| **4. Everything** | Changed config and rotated secrets at the same time |

Options 1, 2, and 4 automatically redeploy so changes take effect immediately.

---

## Other Deployment Methods

- **Local test run** — [guides/local-run.md](guides/local-run.md). Runs on your machine with no persistent storage and no public callback URL. Suitable for verifying configuration; not for production.
- **Raspberry Pi** — [guides/raspberry-pi.md](guides/raspberry-pi.md). Always-on self-hosted option using systemd and Cloudflare Tunnel for callbacks.
```

- [ ] **Step 2.3: Verify the README has the expected top-level sections**

```bash
grep "^## " README.md
```

Expected output (in this order):
```
## Dashboard
## Deploy to Fly.io
## Configuration
## Alarm Priority Reference
## Updating
## Other Deployment Methods
```

- [ ] **Step 2.4: Verify all internal links resolve**

```bash
test -f guides/local-run.md    && echo "local-run.md OK"    || echo "MISSING local-run.md"
test -f guides/raspberry-pi.md && echo "raspberry-pi.md OK" || echo "MISSING raspberry-pi.md"
test -f guides/github-actions.md && echo "github-actions.md OK" || echo "MISSING github-actions.md"
```

Expected: all three print OK.

- [ ] **Step 2.5: Commit**

```bash
git add README.md
git commit -m "docs: restructure README — Fly.io primary path, add dashboard section"
```

---

## Task 3: Trim `guides/fly-io.md`

**Files:**
- Modify: `guides/fly-io.md`

The setup steps (Prerequisites through "Set the callback URL") now live in the README. The guide should retain only the troubleshooting section, with a link back to the README for the full setup flow.

- [ ] **Step 3.1: Read `guides/fly-io.md` in full**

Read the file before editing. The troubleshooting section starts at the `## Troubleshooting` heading and covers: container won't start, Dexcom auth failures, missing secrets, app stops sending readings.

- [ ] **Step 3.2: Replace the file**

Use the Write tool to replace `guides/fly-io.md` with this exact content:

```markdown
# Fly.io Deployment — Troubleshooting

For the full setup guide, see [README → Deploy to Fly.io](../README.md#deploy-to-flyio).

---

## Troubleshooting

**Container won't start**

```bash
fly logs --app <appname>
```

Look for `ERROR: CONFIG_TOML environment variable is required`. Check what secrets are set:

```bash
fly secrets list --app <appname>
```

Re-run `./fly/update.sh` option 1 or 4 to re-upload the config.

**Dexcom auth failures in logs (`session expired` looping)**

Wrong Dexcom username or password. Run `./fly/update.sh` option 2 to re-enter Dexcom credentials.

**A `${VAR}` reference in config has no matching secret**

The app will fail to connect to Dexcom or Pushover. Run `./fly/update.sh` option 4 to re-enter all values.

**App stops sending readings after a while**

Check `fly logs` for fetch errors. If the Dexcom session expired and re-auth is failing, the credentials may have changed. Use option 2.
```

- [ ] **Step 3.3: Verify the file starts with the link back to README**

```bash
head -3 guides/fly-io.md
```

Expected:
```
# Fly.io Deployment — Troubleshooting

For the full setup guide, see [README → Deploy to Fly.io](../README.md#deploy-to-flyio).
```

- [ ] **Step 3.4: Commit and push**

```bash
git add guides/fly-io.md
git commit -m "docs: trim fly-io guide to troubleshooting only"
git push
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task that covers it |
|---|---|
| Fly.io as primary deployment path | Task 2 — README leads with Deploy to Fly.io |
| Dashboard section with widgets + URL | Task 2 — `## Dashboard` section |
| `${VAR}` pattern explained in plain English | Task 2 — callout box in step 2 |
| Hidden input explained | Task 2 — "Nothing appears as you type" bullet |
| Credential-to-prompt mapping table | Task 2 — prompt table in step 3 |
| `guides/local-run.md` with testing-only warning | Task 1 |
| Flags and env vars moved to local-run guide | Task 1 |
| `guides/fly-io.md` trimmed to troubleshooting | Task 3 |
| Link back to README from fly-io.md | Task 3 |
| Other methods (Pi, local) still accessible | Task 2 — `## Other Deployment Methods` |
| CI/CD guide linked from deploy section | Task 2 — "Automate deploys" paragraph |
| Updating section with update.sh table | Task 2 — `## Updating` |
| Configuration section unchanged | Task 2 — copied verbatim |
