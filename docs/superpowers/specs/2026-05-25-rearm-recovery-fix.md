# Rearm-on-Recovery Fix Design

**Date:** 2026-05-25
**Status:** Approved

## Overview

`rearm_on_recovery = true` is meant to reset an alarm's backoff timer when BG recovers past the threshold, so the alarm can fire again immediately on the next crossing. The current implementation achieves this by setting `last_fired_at = NULL` in `alarm_state`. This also erases the dashboard's "Last Fired" display, making the alarm appear as if it never fired even after a real notification was sent.

**Fix:** Add a `rearmed` boolean column to `alarm_state` to separate "backoff reset" from "fire history." `ClearAlarmRearm` sets `rearmed = 1` without touching `last_fired_at`. `shouldFire` skips the backoff check when `rearmed = 1`. `UpdateFiredState` clears `rearmed = 0` when the alarm fires again.

---

## Schema

Add one column to `alarm_state` via `ALTER TABLE` at startup in `store.New()`:

```sql
ALTER TABLE alarm_state ADD COLUMN rearmed INTEGER NOT NULL DEFAULT 0
```

`DEFAULT 0` means all existing rows are `rearmed = false` with no data migration needed. The `ALTER TABLE` is idempotent-guarded: wrapped in a `IF NOT EXISTS`-style check (SQLite does not support `ADD COLUMN IF NOT EXISTS`, so the code catches the "duplicate column" error and ignores it).

---

## Type change

`types.AlarmState` gains one field:

```go
type AlarmState struct {
    ID               int
    Account          string
    AlarmName        string
    Recipient        string
    LastFiredAt      *time.Time
    SnoozedUntil     *time.Time
    ReceiptID        *string
    ReceiptExpiresAt *time.Time
    Rearmed          bool
}
```

No JSON tags — `AlarmState` is not exposed in the API response.

---

## Store changes

### `GetAlarmState`

Add `rearmed` to the `SELECT` list and scan it into `state.Rearmed`:

```go
SELECT id, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed
FROM alarm_state WHERE account = ? AND alarm_name = ? AND recipient = ?
```

Scan: `&state.ID, &lastFiredAt, &snoozedUntil, &rid, &receiptExpires, &state.Rearmed`

### `UpdateFiredState`

Add `rearmed = 0` to the `INSERT` columns and `DO UPDATE SET`:

```go
INSERT INTO alarm_state
    (account, alarm_name, recipient, last_fired_at, receipt_id, receipt_expires_at, rearmed)
VALUES (?, ?, ?, ?, ?, ?, 0)
ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
    last_fired_at      = excluded.last_fired_at,
    receipt_id         = excluded.receipt_id,
    receipt_expires_at = excluded.receipt_expires_at,
    rearmed            = 0
```

### `ClearAlarmRearm`

Change from clearing `last_fired_at` to setting `rearmed = 1`, and keep clearing `snoozed_until`:

```go
UPDATE alarm_state
SET rearmed = 1, snoozed_until = NULL
WHERE account = ? AND alarm_name = ? AND recipient = ?
```

`last_fired_at` is no longer touched.

---

## Evaluator change

`shouldFire` skips the backoff check when `state.Rearmed` is true:

```go
func shouldFire(alarm config.AlarmConfig, state *types.AlarmState, now time.Time) bool {
    if state.SnoozedUntil != nil && now.Before(*state.SnoozedUntil) {
        return false
    }
    if state.ReceiptID != nil && state.ReceiptExpiresAt != nil && now.Before(*state.ReceiptExpiresAt) {
        return false
    }
    if !state.Rearmed && alarm.Priority != "emergency" && alarm.Backoff != "" && state.LastFiredAt != nil {
        backoff, err := time.ParseDuration(alarm.Backoff)
        if err == nil && now.Sub(*state.LastFiredAt) < backoff {
            return false
        }
    }
    return true
}
```

The only change is wrapping the backoff block with `if !state.Rearmed`.

---

## Dashboard

No changes. `last_fired_at` is now always preserved, so the dashboard correctly shows "last fired X hours ago" after a rearm-on-recovery cycle.

---

## Tests

### `store/store_test.go`

Update existing `ClearAlarmRearm` test (or add a targeted case) to assert:
- `last_fired_at` is **preserved** after `ClearAlarmRearm`
- `rearmed` is `1` after `ClearAlarmRearm`
- `rearmed` is `0` after a subsequent `UpdateFiredState`

### `evaluator/evaluator_test.go`

New test: alarm with `rearm_on_recovery = true`, state has `LastFiredAt` set within the backoff window and `Rearmed = true` → `shouldFire` returns `true`.

Existing rearm tests: verify they still pass with the updated `AlarmState` struct (adding `Rearmed: true` where the rearm path is exercised).

---

## Out of Scope

- Displaying "rearmed" status in the dashboard UI
- Per-alarm rearm history or audit log
