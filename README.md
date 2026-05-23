# dexmon

A long-running Go daemon that polls one or more Dexcom Share CGM accounts, evaluates configurable alarm rules, and delivers per-recipient Pushover notifications. Runs on a Raspberry Pi, any ARM host, or a free-tier cloud instance.

## Features

- Multiple Dexcom Share accounts, each with independent alarm rules
- Per-alarm threshold, trend direction filter, and priority (normal / high / emergency)
- Emergency alarms retry until acknowledged via Pushover; acknowledgments are received by webhook
- Per-recipient snooze: one person snoozing does not silence others
- Backoff and rearm-on-recovery controls to avoid alert fatigue
- Dead man's switch via healthchecks.io (or any ping URL)
- Built-in missed-readings health alarm
- Pure-Go SQLite (no CGO) — cross-compiles to ARM without a toolchain

---

## Quick Start

### 1. Build

```bash
go build -o dexmon .
```

The binary has no runtime dependencies.

### 2. Configure

Copy the example config and edit it:

```bash
cp config.toml.example config.toml
```

Edit `config.toml` to match your setup (see [Configuration](#configuration) below). Secrets are never stored in the config file — they are injected via environment variables.

### 3. Set environment variables

```bash
export PUSHOVER_APP_TOKEN=...
export PUSHOVER_USER_KEY_BRANDON=...
export PUSHOVER_USER_KEY_SARAH=...
export DEXCOM_USER_JESSICA=...
export DEXCOM_PASS_JESSICA=...
export HEALTHCHECKS_PING_URL=...   # optional
```

### 4. Run

```bash
./dexmon -config config.toml -db dexmon.db
```

---

## Configuration

All configuration lives in a single TOML file. Secrets are referenced as `${ENV_VAR_NAME}` and are expanded from the environment at startup — the literal `${...}` tokens are never written to the config file.

### `[server]`

```toml
[server]
callback_port = 8080
callback_url  = "https://your-domain.com/pushover/callback"
```

| Key | Description |
|---|---|
| `callback_port` | Local port the webhook server listens on |
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

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `PUSHOVER_APP_TOKEN` | Yes | Pushover application token (from pushover.net → Your Apps) |
| `PUSHOVER_USER_KEY_<NAME>` | Per recipient | Pushover user key for each recipient defined in config |
| `DEXCOM_USER_<NAME>` | Per account | Dexcom Share username for each account |
| `DEXCOM_PASS_<NAME>` | Per account | Dexcom Share password for each account |
| `HEALTHCHECKS_PING_URL` | No | Watchdog ping URL (referenced via `${...}` in config) |

The `<NAME>` suffix in variable names is the token referenced in `config.toml`. The names are arbitrary — they just need to match between the config file and the environment.

---

## Deployment

### Raspberry Pi (systemd)

1. Build for ARM:

   ```bash
   GOOS=linux GOARCH=arm64 go build -o dexmon .
   ```

   Or on the Pi itself: `go build -o dexmon .`

2. Copy files to the host:

   ```bash
   scp dexmon config.toml pi@yourpi:/opt/dexmon/
   ```

3. Create a secrets file (mode 600, root-only):

   ```bash
   sudo tee /opt/dexmon/secrets.env > /dev/null <<EOF
   PUSHOVER_APP_TOKEN=...
   PUSHOVER_USER_KEY_BRANDON=...
   PUSHOVER_USER_KEY_SARAH=...
   DEXCOM_USER_JESSICA=...
   DEXCOM_PASS_JESSICA=...
   HEALTHCHECKS_PING_URL=...
   EOF
   sudo chmod 600 /opt/dexmon/secrets.env
   sudo chown root:root /opt/dexmon/secrets.env
   ```

4. Create a system user:

   ```bash
   sudo useradd --system --no-create-home dexmon
   sudo chown dexmon:dexmon /opt/dexmon
   ```

5. Install the systemd unit:

   ```bash
   sudo cp dexmon.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now dexmon
   ```

6. Check logs:

   ```bash
   journalctl -u dexmon -f
   ```

### Pushover Callback (for emergency acknowledgment)

Emergency alarms support acknowledgment and snooze via Pushover callback. Pushover POSTs to `callback_url` when a recipient acknowledges.

To receive callbacks, `callback_url` must be reachable from the internet. Options:
- **Port forward** port 8080 on your router to the Pi
- **Reverse proxy** with nginx or Caddy on the Pi
- **Cloudflare Tunnel** or similar (no open port required)

If you don't configure a callback URL, emergency alarms still fire and retry — they just can't be acknowledged from the Pushover app.

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

The workflow file is already at `.github/workflows/fly-deploy.yml`. Before enabling CI/CD, make sure you have run `./fly/deploy.sh` at least once and committed the updated `fly/fly.toml` — it must contain your real app name, not the placeholder. To disable CI/CD, delete the workflow file or remove the `FLY_API_TOKEN` secret.

---

## Flags

```
./dexmon -config config.toml -db dexmon.db
```

| Flag | Default | Description |
|---|---|---|
| `-config` | `config.toml` | Path to the TOML config file |
| `-db` | `dexmon.db` | Path to the SQLite database file |

---

## Error Handling

| Failure | Behavior |
|---|---|
| Dexcom API unreachable | Log, increment miss counter, skip tick — never crashes |
| Dexcom session expired | Re-authenticates transparently |
| Pushover API failure | Log, leave alarm state untouched, retry next evaluation |
| SQLite write failure | Fatal — systemd restarts the service |
| Invalid config on startup | Fatal with descriptive error before any network calls |
