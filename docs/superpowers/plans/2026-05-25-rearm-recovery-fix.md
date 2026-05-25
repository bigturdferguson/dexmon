# Rearm-on-Recovery Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `rearm_on_recovery` so it resets the backoff timer on BG recovery without erasing the dashboard's "last fired" history.

**Architecture:** Add a `rearmed` boolean column to `alarm_state`. `ClearAlarmRearm` sets `rearmed = 1` and clears `snoozed_until` but leaves `last_fired_at` intact. `shouldFire` skips the backoff check when `rearmed = 1`. `UpdateFiredState` clears `rearmed = 0` when the alarm fires again.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), `database/sql`

---

## File Map

| File | Change |
|------|--------|
| `types/types.go` | Add `Rearmed bool` to `AlarmState` |
| `store/schema.go` | Add `rearmed` column to `CREATE TABLE alarm_state` |
| `store/store.go` | Add `ALTER TABLE` migration for existing DBs; add `strings` import |
| `store/alarms.go` | Update `GetAlarmState`, `GetAlarmStateByReceiptID`, `UpdateFiredState`, `ClearAlarmRearm` |
| `store/store_test.go` | Update `TestClearAlarmRearm_ClearsLastFiredAndSnooze`; add two new tests |
| `evaluator/evaluator.go` | Update `shouldFire` to skip backoff when `state.Rearmed` |
| `evaluator/evaluator_test.go` | Add `TestEvaluate_FiresImmediatelyAfterRearm` |

---

### Task 1: Type field + schema migration

**Files:**
- Modify: `types/types.go`
- Modify: `store/schema.go`
- Modify: `store/store.go`
- Modify: `store/store_test.go`

- [ ] **Step 1: Write a failing test that references `state.Rearmed`**

Append to `store/store_test.go`:

```go
func TestGetAlarmState_ReturnsRearmedFalseByDefault(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if got.Rearmed {
		t.Error("expected Rearmed=false for fresh state")
	}
}
```

- [ ] **Step 2: Run test to verify it fails to compile**

```bash
cd /home/brandon/Projects/dexmon && go test ./store/ -run TestGetAlarmState_ReturnsRearmedFalseByDefault -v
```

Expected: compile error — `got.Rearmed undefined`

- [ ] **Step 3: Add `Rearmed bool` to `AlarmState` in `types/types.go`**

```go
type AlarmState struct {
	ID               int64
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

- [ ] **Step 4: Add `rearmed` column to `schema.go`**

Replace the `alarm_state` CREATE TABLE:

```go
CREATE TABLE IF NOT EXISTS alarm_state (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    account            TEXT    NOT NULL,
    alarm_name         TEXT    NOT NULL,
    recipient          TEXT    NOT NULL,
    last_fired_at      DATETIME,
    snoozed_until      DATETIME,
    receipt_id         TEXT,
    receipt_expires_at DATETIME,
    rearmed            INTEGER NOT NULL DEFAULT 0,
    UNIQUE (account, alarm_name, recipient)
);
```

- [ ] **Step 5: Add ALTER TABLE migration to `store/store.go`**

The `schema` SQL above creates the column for fresh DBs. For existing DBs the column must be added via migration. SQLite does not support `ADD COLUMN IF NOT EXISTS`, so catch the duplicate-column error:

```go
package store

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Add rearmed column to existing databases. Fresh DBs already have it from
	// schema above; SQLite returns "duplicate column name" in that case — ignore it.
	_, err := s.db.Exec(`ALTER TABLE alarm_state ADD COLUMN rearmed INTEGER NOT NULL DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}
```

- [ ] **Step 6: Run the new test to verify it passes**

```bash
cd /home/brandon/Projects/dexmon && go test ./store/ -run TestGetAlarmState_ReturnsRearmedFalseByDefault -v
```

Expected: PASS

- [ ] **Step 7: Run all tests to verify no regressions**

```bash
cd /home/brandon/Projects/dexmon && go test ./...
```

Expected: all PASS (the `Rearmed` field is zero-value `false` by default, so no existing tests break yet)

- [ ] **Step 8: Commit**

```bash
git add types/types.go store/schema.go store/store.go store/store_test.go
git commit -m "feat: add Rearmed field to AlarmState type and schema"
```

---

### Task 2: Store methods — GetAlarmState, UpdateFiredState, ClearAlarmRearm, GetAlarmStateByReceiptID

**Files:**
- Modify: `store/alarms.go`
- Modify: `store/store_test.go`

- [ ] **Step 1: Update the existing `TestClearAlarmRearm` test to assert the new behavior**

The existing test `TestClearAlarmRearm_ClearsLastFiredAndSnooze` asserts that `LastFiredAt` is nil after `ClearAlarmRearm` — that was the old (broken) behavior. Replace it entirely:

```go
func TestClearAlarmRearm_PreservesLastFiredAtAndSetsRearmed(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	snooze := now.Add(30 * time.Minute)
	rid := "receipt-xyz"
	expires := now.Add(2 * time.Hour)
	state := types.AlarmState{
		Account:          "jessica",
		AlarmName:        "Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		SnoozedUntil:     &snooze,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	}
	if err := s.UpsertAlarmState(state); err != nil {
		t.Fatalf("setup UpsertAlarmState: %v", err)
	}

	if err := s.ClearAlarmRearm("jessica", "Low", "brandon"); err != nil {
		t.Fatalf("ClearAlarmRearm: %v", err)
	}

	got, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState after clear: %v", err)
	}
	if got.LastFiredAt == nil || !got.LastFiredAt.Equal(now) {
		t.Error("expected LastFiredAt preserved after ClearAlarmRearm")
	}
	if !got.Rearmed {
		t.Error("expected Rearmed=true after ClearAlarmRearm")
	}
	if got.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil cleared after ClearAlarmRearm")
	}
	if got.ReceiptID == nil || *got.ReceiptID != rid {
		t.Errorf("expected ReceiptID preserved, got %v", got.ReceiptID)
	}
}
```

Also append a test that `UpdateFiredState` clears the rearmed flag:

```go
func TestUpdateFiredState_ClearsRearmed(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	// Fire, then rearm.
	if err := s.UpdateFiredState("jessica", "Low", "brandon", now, nil, nil); err != nil {
		t.Fatalf("UpdateFiredState: %v", err)
	}
	if err := s.ClearAlarmRearm("jessica", "Low", "brandon"); err != nil {
		t.Fatalf("ClearAlarmRearm: %v", err)
	}
	mid, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if !mid.Rearmed {
		t.Fatal("setup: expected Rearmed=true after ClearAlarmRearm")
	}

	// Fire again — rearmed should clear.
	later := now.Add(time.Hour)
	if err := s.UpdateFiredState("jessica", "Low", "brandon", later, nil, nil); err != nil {
		t.Fatalf("UpdateFiredState (second): %v", err)
	}
	got, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState after second fire: %v", err)
	}
	if got.Rearmed {
		t.Error("expected Rearmed=false after UpdateFiredState")
	}
	if got.LastFiredAt == nil || !got.LastFiredAt.Equal(later) {
		t.Errorf("expected LastFiredAt=%v, got %v", later, got.LastFiredAt)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd /home/brandon/Projects/dexmon && go test ./store/ -run 'TestClearAlarmRearm_PreservesLastFiredAtAndSetsRearmed|TestUpdateFiredState_ClearsRearmed' -v
```

Expected: both FAIL — old `ClearAlarmRearm` still nulls `last_fired_at` and doesn't set `rearmed`

- [ ] **Step 3: Update `store/alarms.go` — all four methods**

Replace the entire file:

```go
package store

import (
	"database/sql"
	"errors"
	"time"

	"dexmon/types"
)

func (s *Store) GetAlarmState(account, alarmName, recipient string) (*types.AlarmState, error) {
	state := &types.AlarmState{
		Account:   account,
		AlarmName: alarmName,
		Recipient: recipient,
	}
	var lastFiredAt, snoozedUntil, receiptExpires sql.NullTime
	var rid sql.NullString
	err := s.db.QueryRow(
		`SELECT id, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed
		 FROM alarm_state WHERE account = ? AND alarm_name = ? AND recipient = ?`,
		account, alarmName, recipient,
	).Scan(&state.ID, &lastFiredAt, &snoozedUntil, &rid, &receiptExpires, &state.Rearmed)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if lastFiredAt.Valid {
		t := lastFiredAt.Time.UTC()
		state.LastFiredAt = &t
	}
	if snoozedUntil.Valid {
		t := snoozedUntil.Time.UTC()
		state.SnoozedUntil = &t
	}
	if rid.Valid {
		state.ReceiptID = &rid.String
	}
	if receiptExpires.Valid {
		t := receiptExpires.Time.UTC()
		state.ReceiptExpiresAt = &t
	}
	return state, nil
}

func (s *Store) GetAlarmStateByReceiptID(receiptID string) (*types.AlarmState, error) {
	var state types.AlarmState
	var lastFiredAt, snoozedUntil, receiptExpires sql.NullTime
	var rid sql.NullString
	err := s.db.QueryRow(
		`SELECT id, account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed
		 FROM alarm_state WHERE receipt_id = ?`,
		receiptID,
	).Scan(&state.ID, &state.Account, &state.AlarmName, &state.Recipient,
		&lastFiredAt, &snoozedUntil, &rid, &receiptExpires, &state.Rearmed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastFiredAt.Valid {
		t := lastFiredAt.Time.UTC()
		state.LastFiredAt = &t
	}
	if snoozedUntil.Valid {
		t := snoozedUntil.Time.UTC()
		state.SnoozedUntil = &t
	}
	if rid.Valid {
		state.ReceiptID = &rid.String
	}
	if receiptExpires.Valid {
		t := receiptExpires.Time.UTC()
		state.ReceiptExpiresAt = &t
	}
	return &state, nil
}

func (s *Store) UpsertAlarmState(state types.AlarmState) error {
	_, err := s.db.Exec(`
		INSERT INTO alarm_state
		    (account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
		    last_fired_at      = excluded.last_fired_at,
		    snoozed_until      = excluded.snoozed_until,
		    receipt_id         = excluded.receipt_id,
		    receipt_expires_at = excluded.receipt_expires_at,
		    rearmed            = excluded.rearmed`,
		state.Account, state.AlarmName, state.Recipient,
		nullTime(state.LastFiredAt),
		nullTime(state.SnoozedUntil),
		nullString(state.ReceiptID),
		nullTime(state.ReceiptExpiresAt),
		state.Rearmed,
	)
	return err
}

// UpdateFiredState sets last_fired_at and optionally receipt_id/receipt_expires_at,
// and clears rearmed. Use this instead of UpsertAlarmState when dispatching.
func (s *Store) UpdateFiredState(account, alarmName, recipient string, lastFiredAt time.Time, receiptID *string, receiptExpiresAt *time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO alarm_state (account, alarm_name, recipient, last_fired_at, receipt_id, receipt_expires_at, rearmed)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
		    last_fired_at      = excluded.last_fired_at,
		    receipt_id         = excluded.receipt_id,
		    receipt_expires_at = excluded.receipt_expires_at,
		    rearmed            = 0`,
		account, alarmName, recipient,
		lastFiredAt.UTC(),
		nullString(receiptID),
		nullTime(receiptExpiresAt),
	)
	return err
}

func (s *Store) ClearAlarmRearm(account, alarmName, recipient string) error {
	_, err := s.db.Exec(
		`UPDATE alarm_state SET rearmed = 1, snoozed_until = NULL
		 WHERE account = ? AND alarm_name = ? AND recipient = ?`,
		account, alarmName, recipient,
	)
	return err
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func nullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
```

- [ ] **Step 4: Run store tests to verify they pass**

```bash
cd /home/brandon/Projects/dexmon && go test ./store/ -v
```

Expected: all PASS

- [ ] **Step 5: Run all tests**

```bash
cd /home/brandon/Projects/dexmon && go test ./...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add store/alarms.go store/store_test.go
git commit -m "fix: ClearAlarmRearm preserves last_fired_at; UpdateFiredState clears rearmed flag"
```

---

### Task 3: Evaluator — skip backoff when rearmed

**Files:**
- Modify: `evaluator/evaluator.go`
- Modify: `evaluator/evaluator_test.go`

- [ ] **Step 1: Write the failing test**

Append to `evaluator/evaluator_test.go`:

```go
func TestEvaluate_FiresImmediatelyAfterRearm(t *testing.T) {
	store := newMockStore()
	recent := time.Now().UTC().Add(-5 * time.Minute) // within the 30m backoff window
	store.set("jessica", "Low", "brandon", &types.AlarmState{
		LastFiredAt: &recent,
		Rearmed:     true,
	})

	reading := types.Reading{Account: "jessica", Value: 65, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 1 {
		t.Errorf("expected alarm to fire when rearmed (backoff should be bypassed), got %d fire results", len(fire))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/brandon/Projects/dexmon && go test ./evaluator/ -run TestEvaluate_FiresImmediatelyAfterRearm -v
```

Expected: FAIL — backoff still blocks the fire because `shouldFire` does not yet check `Rearmed`

- [ ] **Step 3: Update `shouldFire` in `evaluator/evaluator.go`**

Change the function to skip the backoff block when `state.Rearmed` is true:

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

- [ ] **Step 4: Run evaluator tests to verify they pass**

```bash
cd /home/brandon/Projects/dexmon && go test ./evaluator/ -v
```

Expected: all PASS including the new test

- [ ] **Step 5: Run all tests**

```bash
cd /home/brandon/Projects/dexmon && go test ./...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add evaluator/evaluator.go evaluator/evaluator_test.go
git commit -m "fix: skip backoff check in shouldFire when alarm state is rearmed"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `rearmed INTEGER NOT NULL DEFAULT 0` column added | Task 1 |
| `Rearmed bool` field in `types.AlarmState` | Task 1 |
| `ALTER TABLE` migration for existing DBs | Task 1 |
| `GetAlarmState` scans `rearmed` | Task 2 |
| `GetAlarmStateByReceiptID` scans `rearmed` | Task 2 |
| `UpdateFiredState` sets `rearmed = 0` | Task 2 |
| `ClearAlarmRearm` sets `rearmed = 1`, clears `snoozed_until`, preserves `last_fired_at` | Task 2 |
| `shouldFire` skips backoff when `state.Rearmed` | Task 3 |
| Store test: `last_fired_at` preserved after `ClearAlarmRearm` | Task 2 |
| Store test: `rearmed` cleared after `UpdateFiredState` | Task 2 |
| Evaluator test: fires immediately when `Rearmed=true` within backoff window | Task 3 |

**Placeholder scan:** None.

**Type consistency:** `state.Rearmed` (bool) defined in Task 1, used identically in Tasks 2 and 3.
