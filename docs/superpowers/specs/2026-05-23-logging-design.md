# Logging Design

**Date:** 2026-05-23
**Status:** Approved

## Overview

Add structured plain-text logs to dexmon covering every Dexcom poll attempt, Pushover alarm dispatch, emergency acknowledgment, healthcheck ping, and deployment script execution. Secrets are always redacted (names logged, never values). Style matches the existing `log.Printf` pattern throughout the codebase.

---

## Go Runtime Logging

### `poller/poller.go` — every `Tick()`

Log every poll attempt regardless of outcome. Reading age is `time.Since(reading.RecordedAt).Round(time.Second)`.

**New reading:**
```
[noah] poll: BG 142 ↗ 4m ago
```

**Duplicate (already seen):**
```
[noah] poll: BG 142 ↗ 9m ago (already seen)
```

**No reading returned (nil, no error):**
```
[noah] poll: no reading returned
```

**Fetch error** — already logged, keep as-is:
```
[noah] fetch error (2 consecutive): <error>
```

**Alarm rearmed on recovery** (in `for _, result := range toRearm` loop):
```
[noah] alarm "Low" rearmed for brandon
```

### `dispatcher/dispatcher.go` — after successful `http.Post`

Log after `result.Status == 1` is confirmed, before `store.UpdateFiredState`.

**Normal priority:**
```
[noah] alarm "Low" fired → brandon (high)
```

**Emergency with receipt:**
```
[noah] alarm "Urgent Low" fired → brandon (emergency, receipt abc1f3d2)
```

Fields: `req.Account`, `req.AlarmName`, `req.Recipient`, `req.Alarm.Priority`, `result.Receipt` (only if non-empty).

### `callback/server.go` — after successful `UpsertAlarmState`

**Without snooze:**
```
callback: noah/"Urgent Low"/brandon acknowledged
```

**With snooze:**
```
callback: noah/"Urgent Low"/brandon acknowledged, snoozed 30m0s
```

Fields: `state.Account`, `state.AlarmName`, `state.Recipient`. Snooze printed only when `payload.Snooze > 0` as `time.Duration(payload.Snooze) * time.Second`.

### `health/health.go` — after successful watchdog ping

```
watchdog ping ok
```

Added after `io.Copy(io.Discard, resp.Body)` in `PingWatchdog`.

---

## Deployment Script Logging

Applies to both `fly/deploy.sh` and `fly/update.sh`. Uses the existing `info()` helper (green `==>` prefix).

### Per-step additions

**After app name / region confirmed** (`deploy.sh`):
```
==> App: my-dexmon  Region: iad
```

**After config path resolved** (both scripts):
```
==> Config: /home/brandon/config.toml
```

**After scanning `${VAR}` references** (both scripts) — names only, never values:
```
==> Secrets to set: PUSHOVER_APP_TOKEN PUSHOVER_USER_KEY_BRANDON DEXCOM_USER_NOAH DEXCOM_PASS_NOAH
```

**When a secret is skipped** (blank entry in `update.sh`):
```
==> Skipping DEXCOM_PASS_NOAH (no value entered)
```

**Choice confirmed** (`update.sh`):
```
==> Option 1: re-encoding config
==> Option 2: updating secrets
==> Option 3: code-only deploy
==> Option 4: updating config and secrets
```

### Full app state summary — printed at end of every run (both scripts)

Printed after all operations complete, before exit. Callback URL is parsed from the `callback_url =` line in config.toml. Secrets list comes from `${VAR}` scan of config.toml.

```
==> App state:
      App:          my-dexmon
      Config:       /home/brandon/config.toml
      Callback URL: https://my-dexmon.fly.dev/pushover/callback
      Secrets:      PUSHOVER_APP_TOKEN PUSHOVER_USER_KEY_BRANDON
                    DEXCOM_USER_NOAH DEXCOM_PASS_NOAH
```

For `update.sh` option 3 (code-only), there is no config file path — omit Config and Secrets lines; show App and the URL parsed from the existing `fly.toml` `app =` field instead.

---

## Files Changed

| File | Change |
|---|---|
| `poller/poller.go` | Add poll result log + rearm log |
| `dispatcher/dispatcher.go` | Add dispatch success log |
| `callback/server.go` | Add acknowledgment log |
| `health/health.go` | Add ping success log |
| `fly/deploy.sh` | Add per-step info + full state summary |
| `fly/update.sh` | Add per-step info + skip logs + full state summary |

---

## What Is Not Logged

- Secret values (ever)
- Evaluator internals (threshold comparisons per alarm — too verbose)
- Store queries
- Duplicate poll readings do get a log line (by design — every attempt is visible)
