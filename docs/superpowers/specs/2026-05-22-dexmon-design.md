# dexmon — Design Spec

**Date:** 2026-05-22
**Status:** Approved

## Overview

`dexmon` is a long-running Go daemon that polls one or more Dexcom Share CGM accounts, evaluates configurable alarm rules against incoming blood glucose readings, and delivers per-recipient notifications via Pushover. Designed to run on a Raspberry Pi or free-tier cloud host (ARM-compatible).

---

## Architecture

A single Go binary with five internal components:

```
┌─────────────────────────────────────────────────────┐
│                    dexmon daemon                     │
│                                                      │
│  ┌──────────┐    ┌───────────┐    ┌───────────────┐ │
│  │  Poller  │───▶│ Evaluator │───▶│  Dispatcher   │ │
│  │(per acct)│    │           │    │ (Pushover API) │ │
│  └──────────┘    └───────────┘    └───────────────┘ │
│        │               ▲                  │          │
│    readings             │           receipt IDs      │
│        │                                  │          │
│        ▼                                  ▼          │
│  ┌─────────────────────────────────────────────────┐ │
│  │                  SQLite Store                   │ │
│  │  (readings, alarm state, per-recipient snooze)  │ │
│  └─────────────────────────────────────────────────┘ │
│                          ▲                           │
│  ┌───────────────────────┴─────────────────────────┐ │
│  │           Callback HTTP Server                  │ │
│  │       (Pushover acknowledgments)                │ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

---

## Configuration

Non-sensitive settings live in a TOML config file. Secrets are injected via environment variables. The convention for env var naming is `<SECRET_TYPE>_<CONFIG_KEY>` where `CONFIG_KEY` matches the relevant TOML section name.

### Config File (config.toml)

```toml
[server]
callback_port = 8080
callback_url  = "https://your-domain.com/pushover/callback"

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3           # alert after N consecutive failed polls
  priority            = "emergency"
  recipients          = ["brandon"]

  [health.watchdog]
  ping_url = "${HEALTHCHECKS_PING_URL}"  # dead man's switch ping URL

[recipients]
  [recipients.brandon]
  pushover_user_key = "${PUSHOVER_USER_KEY_BRANDON}"

  [recipients.sarah]
  pushover_user_key = "${PUSHOVER_USER_KEY_SARAH}"

  [recipients.jessica]
  pushover_user_key = "${PUSHOVER_USER_KEY_JESSICA}"

[accounts]
  [accounts.jessica]
  dexcom_username = "${DEXCOM_USER_JESSICA}"
  dexcom_password = "${DEXCOM_PASS_JESSICA}"
  poll_interval   = "5m"

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

  [[accounts.jessica.alarms]]
  name              = "Low"
  threshold         = 70
  direction         = "below"
  trend             = ["double_down", "single_down", "forty_five_down"]
  priority          = "high"
  backoff           = "30m"
  rearm_on_recovery = true
  recipients        = ["brandon", "sarah"]

  [[accounts.jessica.alarms]]
  name              = "High"
  threshold         = 250
  direction         = "above"
  trend             = ["single_up", "double_up", "forty_five_up"]
  priority          = "normal"
  backoff           = "60m"
  rearm_on_recovery = false
  recipients        = ["brandon", "sarah"]
```

### Environment Variables

```
PUSHOVER_APP_TOKEN=...
PUSHOVER_USER_KEY_BRANDON=...
PUSHOVER_USER_KEY_SARAH=...
PUSHOVER_USER_KEY_JESSICA=...
DEXCOM_USER_JESSICA=...
DEXCOM_PASS_JESSICA=...
HEALTHCHECKS_PING_URL=...
```

---

## SQLite Data Model

### `readings`

Rolling history of BG values per account.

```sql
CREATE TABLE readings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account     TEXT    NOT NULL,
    value       INTEGER NOT NULL,
    trend       TEXT    NOT NULL,
    recorded_at DATETIME NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_readings_account_time ON readings (account, recorded_at DESC);
```

Valid `trend` values (sourced from Dexcom Share API):

| Value | Arrow | Meaning |
|---|---|---|
| `double_up` | ↑↑ | Rising rapidly |
| `single_up` | ↑ | Rising |
| `forty_five_up` | ↗ | Rising slightly |
| `flat` | → | Stable |
| `forty_five_down` | ↘ | Falling slightly |
| `single_down` | ↓ | Falling |
| `double_down` | ↓↓ | Falling rapidly |
| `not_computable` | ? | Sensor cannot compute |
| `rate_out_of_range` | - | Rate too extreme to classify |
| `none` | | No trend data |

Readings older than the configured retention period (default: 30 days) are pruned on startup.

### `alarm_state`

Per-alarm per-recipient state. One row per unique (account, alarm, recipient) combination.

```sql
CREATE TABLE alarm_state (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    account            TEXT    NOT NULL,
    alarm_name         TEXT    NOT NULL,
    recipient          TEXT    NOT NULL,
    last_fired_at      DATETIME,
    snoozed_until      DATETIME,
    receipt_id         TEXT,
    receipt_expires_at DATETIME,
    UNIQUE (account, alarm_name, recipient)
);
```

- `last_fired_at`: updated by Dispatcher on each successful send; used for backoff checks
- `snoozed_until`: set by Callback Server when a Pushover acknowledgment includes a snooze duration
- `receipt_id`: active Pushover Emergency receipt ID; cleared on acknowledgment or expiry
- `receipt_expires_at`: set to `now + expire` when an Emergency is sent; Evaluator treats the receipt as cleared if `now > receipt_expires_at`, preventing a never-acknowledged expired receipt from permanently blocking new sends

---

## Component Details

### Poller

- One goroutine per configured account
- Authenticates with Dexcom Share API on startup to obtain a session token
- Re-authenticates transparently on token expiry (~6 hours)
- On each tick: fetches latest reading, deduplicates by `recorded_at`, inserts if new, passes to Evaluator
- On consecutive failures reaching `max_missed_readings`: triggers a health alarm via Dispatcher
- On each successful poll: pings `health.watchdog.ping_url` (dead man's switch)
- API unreachable: logs error, increments failure counter, skips evaluation — never crashes

### Evaluator

Called synchronously per new reading. For each alarm configured on the account:

1. Check value crosses threshold in configured direction
2. Check trend is in the alarm's trend filter list
3. For each recipient:
   - Skip if `snoozed_until` is in the future
   - Skip if `now - last_fired_at < backoff` (non-emergency)
   - Skip if `receipt_id` is non-null AND `receipt_expires_at` is in the future (active unacknowledged emergency)
4. Pass recipients that clear all checks to Dispatcher
5. On recovery (BG crosses back through threshold): if `rearm_on_recovery = true`, clear `last_fired_at` and `snoozed_until` for all recipients of that alarm

**Backoff behavior:** A sequence of readings below threshold (e.g. 59→58→57) during an active backoff fires exactly once. If BG recovers above the threshold and drops again:
- `rearm_on_recovery = true`: backoff resets, alarm is eligible to fire again
- `rearm_on_recovery = false`: original backoff duration is honored regardless of recovery

### Dispatcher

- Sends individual Pushover notifications per recipient (never group sends)
- Priority mapping:

| Alarm priority | Pushover priority |
|---|---|
| `emergency` | 2 (retries until acknowledged) |
| `high` | 1 (bypasses quiet hours) |
| `normal` | 0 |

- Emergency sends include: `retry`, `expire`, and `callback` URL
- On success: writes `last_fired_at` and `receipt_id` (emergency only) to `alarm_state`
- On Pushover API failure: logs error, leaves `alarm_state` untouched so next evaluation retries

### Callback HTTP Server

- Listens on `server.callback_port`
- Handles `POST /pushover/callback` from Pushover
- Payload includes: `receipt`, `acknowledged_at`, `snooze` duration (if set by user)
- Looks up `receipt_id` in `alarm_state`
- If snooze set: `snoozed_until = now + snooze`, clears `receipt_id`
- If no snooze: clears `receipt_id` only, allowing alarm to re-arm normally
- Snooze is per-recipient — one recipient acknowledging does not affect others

---

## Health Monitoring

Two complementary mechanisms:

**Internal (missed readings alarm)**
- Poller tracks consecutive failed polls per account
- On reaching `max_missed_readings`, fires a Pushover alert via Dispatcher using the configured priority and recipients
- Resets counter on next successful reading

**External dead man's switch**
- Poller pings `health.watchdog.ping_url` after each successful poll cycle
- If pings stop, the external service (e.g. healthchecks.io, free tier) sends its own alert
- Covers cases where the process crashes or the host goes down entirely

---

## Error Handling

| Failure | Behavior |
|---|---|
| Dexcom API unreachable | Log, increment miss counter, skip tick |
| Dexcom session expired | Re-authenticate transparently |
| Pushover API failure | Log, leave alarm state untouched, retry next evaluation |
| Pushover callback unreachable | Pushover retries callback for ~1 hour |
| SQLite write failure | Fatal — restart the service |
| Invalid config on startup | Fatal with descriptive error before any network calls |

---

## Testing

- **Unit tests**: Evaluator logic — threshold crossing, trend filtering, backoff/snooze checks, rearm-on-recovery behavior. Pure functions, no I/O, table-driven.
- **Integration tests**: Poller deduplication and Dispatcher state writes against SQLite using `file::memory:?cache=shared` — real schema, real queries, no mocks.
- **Mock HTTP server**: Pushover API calls tested against a local mock server to verify correct priority, retry, and callback parameters.
- SQLite is never mocked.

---

## Deployment

**Raspberry Pi (systemd)**
- Secrets set in the systemd unit file (`EnvironmentFile`, mode 600, root-only)
- Service configured with `Restart=always`
- Callback server requires port forwarding or a reverse proxy (e.g. nginx, Caddy) if receiving Pushover callbacks from the internet

**Free cloud (Fly.io, Render, Railway)**
- Secrets set via provider env var mechanism
- Callback URL is the provider's public URL
- SQLite volume must be persistent (not ephemeral storage)
