# dexmon

A long-running Go daemon that polls one or more Dexcom Share CGM accounts, evaluates configurable alarm rules, and delivers per-recipient Pushover notifications. Runs on a Raspberry Pi, any ARM host, or a free-tier cloud instance.

---

## How It Works

dexmon runs a polling loop for each configured Dexcom account. Every `poll_interval`, it fetches the latest CGM reading from the Dexcom Share API. Each reading is evaluated against the account's alarm rules — an alarm fires when the blood glucose value crosses a threshold **and** the current trend matches the configured filter. Fired alarms are dispatched as Pushover notifications at the configured priority. Emergency-priority alarms retry until acknowledged; when a recipient acknowledges from the Pushover app, Pushover POSTs to dexmon's callback URL, which stops retrying and optionally snoozes for that recipient.

---

## Features

- Real-time web dashboard at `/` — current BG, stats, 24-hour graph, alarm summary; auto-refreshes every 5 minutes; light/dark theme
- Multiple Dexcom Share accounts, each with independent alarm rules (dashboard displays one account)
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

| Platform | Guide |
|---|---|
| Raspberry Pi (home, always-on) | [guides/raspberry-pi.md](guides/raspberry-pi.md) |
| Fly.io (cloud, no hardware needed) | [guides/fly-io.md](guides/fly-io.md) |
| GitHub Actions CI/CD (auto-deploy on push) | [guides/github-actions.md](guides/github-actions.md) |

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
