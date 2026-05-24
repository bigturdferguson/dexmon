# dexmon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go daemon that polls Dexcom Share CGM accounts, evaluates configurable alarm rules, and sends per-recipient Pushover notifications with emergency receipt/snooze handling.

**Architecture:** Single binary with five components (Poller, Evaluator, Dispatcher, Callback Server, SQLite Store) wired in `main.go`. The polling ticker is the sole driver — Poller calls Evaluator synchronously, Evaluator calls Dispatcher for each qualifying recipient. No channels or message queues.

**Tech Stack:** Go 1.22+, `modernc.org/sqlite` (pure-Go SQLite, no CGO, ARM-compatible), `github.com/BurntSushi/toml` (TOML config), standard `net/http` (Dexcom, Pushover, callback server).

---

## File Structure

```
dexmon/
├── main.go                    # entry point: parse flags, load config, wire, run
├── go.mod
├── config/
│   ├── config.go              # Config structs, Load(), expandEnv(), validate()
│   └── config_test.go
├── types/
│   └── types.go               # Reading, AlarmState, Trend constants
├── store/
│   ├── store.go               # Store struct, New(), Close(), migrate()
│   ├── schema.go              # SQL schema strings (CREATE TABLE IF NOT EXISTS)
│   ├── readings.go            # InsertReading(), HasReading(), PruneReadings()
│   ├── alarms.go              # GetAlarmState(), UpsertAlarmState(), GetAlarmStateByReceiptID(), ClearAlarmRearm()
│   └── store_test.go
├── dexcom/
│   ├── client.go              # Client struct, Login(), FetchLatest(), fetchLatestRaw()
│   └── client_test.go
├── evaluator/
│   ├── evaluator.go           # Evaluate() pure function — no I/O, uses AlarmStateReader interface
│   └── evaluator_test.go
├── dispatcher/
│   ├── dispatcher.go          # Dispatcher struct, Send()
│   └── dispatcher_test.go
├── poller/
│   ├── poller.go              # Poller struct, New(), Run(), tick()
│   └── poller_test.go
├── callback/
│   ├── server.go              # Server struct, Start(), handleCallback()
│   └── server_test.go
└── health/
    └── health.go              # PingWatchdog(), FireMissedReadingsAlarm()
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize module**

```bash
cd /home/brandon/Projects/dexmon
go mod init dexmon
```

Expected: `go.mod` created with `module dexmon` and `go 1.22`.

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/BurntSushi/toml@latest
go get modernc.org/sqlite@latest
go mod tidy
```

Expected: `go.mod` and `go.sum` updated with both packages.

- [ ] **Step 3: Create package directories**

```bash
mkdir -p config types store dexcom evaluator dispatcher poller callback health
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: initialize Go module with dependencies"
```

---

### Task 2: Shared Types

**Files:**
- Create: `types/types.go`

- [ ] **Step 1: Write types/types.go**

```go
package types

import "time"

type Trend string

const (
	TrendDoubleUp       Trend = "double_up"
	TrendSingleUp       Trend = "single_up"
	TrendFortyFiveUp    Trend = "forty_five_up"
	TrendFlat           Trend = "flat"
	TrendFortyFiveDown  Trend = "forty_five_down"
	TrendSingleDown     Trend = "single_down"
	TrendDoubleDown     Trend = "double_down"
	TrendNotComputable  Trend = "not_computable"
	TrendRateOutOfRange Trend = "rate_out_of_range"
	TrendNone           Trend = "none"
)

type Reading struct {
	Account    string
	Value      int
	Trend      Trend
	RecordedAt time.Time
}

type AlarmState struct {
	ID               int64
	Account          string
	AlarmName        string
	Recipient        string
	LastFiredAt      *time.Time
	SnoozedUntil     *time.Time
	ReceiptID        *string
	ReceiptExpiresAt *time.Time
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./types/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add types/
git commit -m "feat: add shared types package"
```

---

### Task 3: Config

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`

- [ ] **Step 1: Write failing tests**

```go
// config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestLoad_ExpandsEnvVars(t *testing.T) {
	t.Setenv("TEST_USER_KEY", "ukey123")
	t.Setenv("DEXCOM_USER", "jess@example.com")
	t.Setenv("DEXCOM_PASS", "secret")
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = "https://example.com/cb"

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = ["brandon"]
  [health.watchdog]
  ping_url = ""

[recipients]
  [recipients.brandon]
  pushover_user_key = "${TEST_USER_KEY}"

[accounts]
  [accounts.jessica]
  dexcom_username = "${DEXCOM_USER}"
  dexcom_password = "${DEXCOM_PASS}"
  poll_interval   = "5m"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Recipients["brandon"].PushoverUserKey != "ukey123" {
		t.Errorf("got %q, want %q", cfg.Recipients["brandon"].PushoverUserKey, "ukey123")
	}
	if cfg.Accounts["jessica"].DexcomUsername != "jess@example.com" {
		t.Errorf("got %q, want %q", cfg.Accounts["jessica"].DexcomUsername, "jess@example.com")
	}
}

func TestLoad_RejectsUnknownRecipient(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]
  [recipients.brandon]
  pushover_user_key = "ukey"

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"

  [[accounts.jessica.alarms]]
  name       = "Low"
  threshold  = 70
  direction  = "below"
  trend      = ["flat"]
  priority   = "normal"
  recipients = ["nobody"]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown recipient, got nil")
	}
}

func TestLoad_RejectsInvalidPollInterval(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "not-a-duration"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid poll_interval, got nil")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./config/... -v
```

Expected: compilation error (`Load` undefined).

- [ ] **Step 3: Write config/config.go**

```go
package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server     ServerConfig               `toml:"server"`
	Health     HealthConfig               `toml:"health"`
	Recipients map[string]RecipientConfig `toml:"recipients"`
	Accounts   map[string]AccountConfig   `toml:"accounts"`
}

type ServerConfig struct {
	CallbackPort int    `toml:"callback_port"`
	CallbackURL  string `toml:"callback_url"`
}

type HealthConfig struct {
	DexcomTimeout DexcomTimeoutConfig `toml:"dexcom_timeout"`
	Watchdog      WatchdogConfig      `toml:"watchdog"`
}

type DexcomTimeoutConfig struct {
	MaxMissedReadings int      `toml:"max_missed_readings"`
	Priority          string   `toml:"priority"`
	Recipients        []string `toml:"recipients"`
}

type WatchdogConfig struct {
	PingURL string `toml:"ping_url"`
}

type RecipientConfig struct {
	PushoverUserKey string `toml:"pushover_user_key"`
}

type AccountConfig struct {
	DexcomUsername string        `toml:"dexcom_username"`
	DexcomPassword string        `toml:"dexcom_password"`
	PollInterval   string        `toml:"poll_interval"`
	Alarms         []AlarmConfig `toml:"alarms"`
}

type AlarmConfig struct {
	Name            string   `toml:"name"`
	Threshold       int      `toml:"threshold"`
	Direction       string   `toml:"direction"`
	Trend           []string `toml:"trend"`
	Priority        string   `toml:"priority"`
	Retry           string   `toml:"retry"`
	Expire          string   `toml:"expire"`
	Backoff         string   `toml:"backoff"`
	RearmOnRecovery bool     `toml:"rearm_on_recovery"`
	Recipients      []string `toml:"recipients"`
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	expanded := envVarRe.ReplaceAllStringFunc(string(data), func(match string) string {
		key := envVarRe.FindStringSubmatch(match)[1]
		return os.Getenv(key)
	})
	var cfg Config
	if _, err := toml.Decode(expanded, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

func validate(cfg *Config) error {
	for name, acct := range cfg.Accounts {
		if acct.DexcomUsername == "" {
			return fmt.Errorf("account %q: dexcom_username required", name)
		}
		if acct.DexcomPassword == "" {
			return fmt.Errorf("account %q: dexcom_password required", name)
		}
		if _, err := time.ParseDuration(acct.PollInterval); err != nil {
			return fmt.Errorf("account %q: invalid poll_interval %q: %w", name, acct.PollInterval, err)
		}
		for _, alarm := range acct.Alarms {
			if alarm.Direction != "above" && alarm.Direction != "below" {
				return fmt.Errorf("account %q, alarm %q: direction must be 'above' or 'below'", name, alarm.Name)
			}
			if alarm.Priority != "emergency" && alarm.Priority != "high" && alarm.Priority != "normal" {
				return fmt.Errorf("account %q, alarm %q: priority must be emergency/high/normal", name, alarm.Name)
			}
			for _, r := range alarm.Recipients {
				if _, ok := cfg.Recipients[r]; !ok {
					return fmt.Errorf("account %q, alarm %q: unknown recipient %q", name, alarm.Name, r)
				}
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./config/... -v
```

Expected: all three tests pass.

- [ ] **Step 5: Commit**

```bash
git add config/
git commit -m "feat: add config parsing with env var expansion and validation"
```

---

### Task 4: Store — Schema and Base

**Files:**
- Create: `store/store.go`
- Create: `store/schema.go`

- [ ] **Step 1: Write store/schema.go**

```go
package store

const schema = `
CREATE TABLE IF NOT EXISTS readings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account     TEXT    NOT NULL,
    value       INTEGER NOT NULL,
    trend       TEXT    NOT NULL,
    recorded_at DATETIME NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_readings_account_time ON readings (account, recorded_at DESC);

CREATE TABLE IF NOT EXISTS alarm_state (
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
`
```

- [ ] **Step 2: Write store/store.go**

```go
package store

import (
	"database/sql"

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
	_, err := s.db.Exec(schema)
	return err
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./store/...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add store/store.go store/schema.go
git commit -m "feat: add SQLite store with schema migration"
```

---

### Task 5: Store — Readings

**Files:**
- Create: `store/readings.go`
- Create: `store/store_test.go` (readings section)

- [ ] **Step 1: Write failing tests**

```go
// store/store_test.go
package store_test

import (
	"testing"
	"time"

	"dexmon/store"
	"dexmon/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInsertReading_NewReading(t *testing.T) {
	s := newTestStore(t)
	ts := time.Now().UTC().Truncate(time.Second)
	r := types.Reading{Account: "jessica", Value: 85, Trend: types.TrendFlat, RecordedAt: ts}

	if err := s.InsertReading(r); err != nil {
		t.Fatalf("InsertReading: %v", err)
	}

	exists, err := s.HasReading("jessica", ts)
	if err != nil {
		t.Fatalf("HasReading: %v", err)
	}
	if !exists {
		t.Error("expected reading to exist after insert")
	}
}

func TestHasReading_ReturnsFalseForMissing(t *testing.T) {
	s := newTestStore(t)
	ts := time.Now().UTC()

	exists, err := s.HasReading("jessica", ts)
	if err != nil {
		t.Fatalf("HasReading: %v", err)
	}
	if exists {
		t.Error("expected no reading before any insert")
	}
}

func TestPruneReadings_RemovesOldRecords(t *testing.T) {
	s := newTestStore(t)
	old := time.Now().UTC().Add(-40 * 24 * time.Hour)
	recent := time.Now().UTC().Add(-1 * time.Hour)

	_ = s.InsertReading(types.Reading{Account: "jessica", Value: 80, Trend: types.TrendFlat, RecordedAt: old})
	_ = s.InsertReading(types.Reading{Account: "jessica", Value: 90, Trend: types.TrendFlat, RecordedAt: recent})

	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if err := s.PruneReadings("jessica", cutoff); err != nil {
		t.Fatalf("PruneReadings: %v", err)
	}

	if exists, _ := s.HasReading("jessica", old); exists {
		t.Error("old reading should have been pruned")
	}
	if exists, _ := s.HasReading("jessica", recent); !exists {
		t.Error("recent reading should not have been pruned")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./store/... -v
```

Expected: compilation error (`InsertReading` etc. undefined).

- [ ] **Step 3: Write store/readings.go**

```go
package store

import (
	"time"

	"dexmon/types"
)

func (s *Store) InsertReading(r types.Reading) error {
	_, err := s.db.Exec(
		`INSERT INTO readings (account, value, trend, recorded_at) VALUES (?, ?, ?, ?)`,
		r.Account, r.Value, string(r.Trend), r.RecordedAt.UTC(),
	)
	return err
}

func (s *Store) HasReading(account string, recordedAt time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM readings WHERE account = ? AND recorded_at = ?`,
		account, recordedAt.UTC(),
	).Scan(&count)
	return count > 0, err
}

func (s *Store) PruneReadings(account string, before time.Time) error {
	_, err := s.db.Exec(
		`DELETE FROM readings WHERE account = ? AND recorded_at < ?`,
		account, before.UTC(),
	)
	return err
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./store/... -run TestInsertReading -v
go test ./store/... -run TestHasReading -v
go test ./store/... -run TestPruneReadings -v
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add store/readings.go store/store_test.go
git commit -m "feat: add readings store operations"
```

---

### Task 6: Store — Alarm State

**Files:**
- Modify: `store/store_test.go` (add alarm state tests)
- Create: `store/alarms.go`

- [ ] **Step 1: Add alarm state tests to store/store_test.go**

Append to `store/store_test.go`:

```go
func TestUpsertAlarmState_CreatesRow(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	state := types.AlarmState{
		Account:     "jessica",
		AlarmName:   "Low",
		Recipient:   "brandon",
		LastFiredAt: &now,
	}

	if err := s.UpsertAlarmState(state); err != nil {
		t.Fatalf("UpsertAlarmState: %v", err)
	}

	got, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if got.LastFiredAt == nil || !got.LastFiredAt.Equal(now) {
		t.Errorf("LastFiredAt: got %v, want %v", got.LastFiredAt, now)
	}
}

func TestUpsertAlarmState_UpdatesExistingRow(t *testing.T) {
	s := newTestStore(t)
	t1 := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)
	t2 := time.Now().UTC().Truncate(time.Second)

	_ = s.UpsertAlarmState(types.AlarmState{Account: "jessica", AlarmName: "Low", Recipient: "brandon", LastFiredAt: &t1})
	_ = s.UpsertAlarmState(types.AlarmState{Account: "jessica", AlarmName: "Low", Recipient: "brandon", LastFiredAt: &t2})

	got, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if got.LastFiredAt == nil || !got.LastFiredAt.Equal(t2) {
		t.Errorf("expected updated LastFiredAt %v, got %v", t2, got.LastFiredAt)
	}
}

func TestGetAlarmState_ReturnsZeroStateForMissing(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetAlarmState("jessica", "Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if got.LastFiredAt != nil || got.SnoozedUntil != nil || got.ReceiptID != nil {
		t.Error("expected zero state for never-seen alarm")
	}
}

func TestGetAlarmStateByReceiptID(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(2 * time.Hour)
	rid := "receipt-abc-123"
	state := types.AlarmState{
		Account:          "jessica",
		AlarmName:        "Severe Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	}
	_ = s.UpsertAlarmState(state)

	got, err := s.GetAlarmStateByReceiptID(rid)
	if err != nil {
		t.Fatalf("GetAlarmStateByReceiptID: %v", err)
	}
	if got == nil {
		t.Fatal("expected state, got nil")
	}
	if got.Account != "jessica" || got.AlarmName != "Severe Low" {
		t.Errorf("unexpected state: %+v", got)
	}
}

func TestClearAlarmRearm_ClearsLastFiredAndSnooze(t *testing.T) {
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
	_ = s.UpsertAlarmState(state)

	if err := s.ClearAlarmRearm("jessica", "Low", "brandon"); err != nil {
		t.Fatalf("ClearAlarmRearm: %v", err)
	}

	got, _ := s.GetAlarmState("jessica", "Low", "brandon")
	if got.LastFiredAt != nil {
		t.Error("expected LastFiredAt cleared")
	}
	if got.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil cleared")
	}
	// receipt_id preserved — ClearAlarmRearm only clears backoff/snooze
	if got.ReceiptID == nil || *got.ReceiptID != rid {
		t.Errorf("expected ReceiptID preserved, got %v", got.ReceiptID)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./store/... -v
```

Expected: compilation error (`UpsertAlarmState` etc. undefined).

- [ ] **Step 3: Write store/alarms.go**

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
		`SELECT id, last_fired_at, snoozed_until, receipt_id, receipt_expires_at
		 FROM alarm_state WHERE account = ? AND alarm_name = ? AND recipient = ?`,
		account, alarmName, recipient,
	).Scan(&state.ID, &lastFiredAt, &snoozedUntil, &rid, &receiptExpires)
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
		`SELECT id, account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at
		 FROM alarm_state WHERE receipt_id = ?`,
		receiptID,
	).Scan(&state.ID, &state.Account, &state.AlarmName, &state.Recipient,
		&lastFiredAt, &snoozedUntil, &rid, &receiptExpires)
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
		    (account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
		    last_fired_at      = excluded.last_fired_at,
		    snoozed_until      = excluded.snoozed_until,
		    receipt_id         = excluded.receipt_id,
		    receipt_expires_at = excluded.receipt_expires_at`,
		state.Account, state.AlarmName, state.Recipient,
		nullTime(state.LastFiredAt),
		nullTime(state.SnoozedUntil),
		nullString(state.ReceiptID),
		nullTime(state.ReceiptExpiresAt),
	)
	return err
}

func (s *Store) ClearAlarmRearm(account, alarmName, recipient string) error {
	_, err := s.db.Exec(
		`UPDATE alarm_state SET last_fired_at = NULL, snoozed_until = NULL
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
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
```

- [ ] **Step 4: Run all store tests**

```bash
go test ./store/... -v
```

Expected: all 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add store/alarms.go store/store_test.go
git commit -m "feat: add alarm state store operations"
```

---

### Task 7: Dexcom API Client

**Files:**
- Create: `dexcom/client.go`
- Create: `dexcom/client_test.go`

- [ ] **Step 1: Write failing tests**

```go
// dexcom/client_test.go
package dexcom_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dexmon/dexcom"
	"dexmon/types"
)

func TestLogin_StoresSessionID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ShareWebServices/Services/General/LoginPublisherAccountByName" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode("session-abc-123")
	}))
	defer srv.Close()

	c := dexcom.NewWithBase("user", "pass", srv.URL+"/ShareWebServices/Services")
	if err := c.Login(); err != nil {
		t.Fatalf("Login: %v", err)
	}
}

func TestFetchLatest_ReturnsReading(t *testing.T) {
	loginCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ShareWebServices/Services/General/LoginPublisherAccountByName":
			loginCalled = true
			json.NewEncoder(w).Encode("session-xyz")
		case "/ShareWebServices/Services/Publisher/ReadPublisherLatestGlucoseValues":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"WT": "/Date(1700000000000)/", "Value": 92, "Trend": "Flat"},
			})
		}
	}))
	defer srv.Close()

	c := dexcom.NewWithBase("user", "pass", srv.URL+"/ShareWebServices/Services")
	reading, err := c.FetchLatest("jessica")
	if err != nil {
		t.Fatalf("FetchLatest: %v", err)
	}
	if !loginCalled {
		t.Error("expected Login to be called automatically")
	}
	if reading == nil {
		t.Fatal("expected reading, got nil")
	}
	if reading.Value != 92 {
		t.Errorf("Value: got %d, want 92", reading.Value)
	}
	if reading.Trend != types.TrendFlat {
		t.Errorf("Trend: got %q, want %q", reading.Trend, types.TrendFlat)
	}
	wantTime := time.UnixMilli(1700000000000).UTC()
	if !reading.RecordedAt.Equal(wantTime) {
		t.Errorf("RecordedAt: got %v, want %v", reading.RecordedAt, wantTime)
	}
}

func TestFetchLatest_ReauthOnSessionExpiry(t *testing.T) {
	loginCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ShareWebServices/Services/General/LoginPublisherAccountByName":
			loginCount++
			json.NewEncoder(w).Encode("new-session")
		case "/ShareWebServices/Services/Publisher/ReadPublisherLatestGlucoseValues":
			if loginCount < 2 {
				// Simulate expired session
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"WT": "/Date(1700000000000)/", "Value": 100, "Trend": "SingleUp"},
			})
		}
	}))
	defer srv.Close()

	c := dexcom.NewWithBase("user", "pass", srv.URL+"/ShareWebServices/Services")
	reading, err := c.FetchLatest("jessica")
	if err != nil {
		t.Fatalf("FetchLatest after reauth: %v", err)
	}
	if loginCount != 2 {
		t.Errorf("expected 2 logins (initial + reauth), got %d", loginCount)
	}
	if reading == nil || reading.Value != 100 {
		t.Errorf("unexpected reading: %+v", reading)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./dexcom/... -v
```

Expected: compilation error (`dexcom.NewWithBase` undefined).

- [ ] **Step 3: Write dexcom/client.go**

```go
package dexcom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dexmon/types"
)

const (
	defaultBase = "https://share2.dexcom.com/ShareWebServices/Services"
	appID       = "d89443d2-327c-4a6f-89e5-496bbb0317db"
)

type Client struct {
	username  string
	password  string
	base      string
	sessionID string
	http      *http.Client
}

func New(username, password string) *Client {
	return NewWithBase(username, password, defaultBase)
}

func NewWithBase(username, password, base string) *Client {
	return &Client{
		username: username,
		password: password,
		base:     base,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Login() error {
	body, _ := json.Marshal(map[string]string{
		"accountName":   c.username,
		"password":      c.password,
		"applicationId": appID,
	})
	resp, err := c.http.Post(
		c.base+"/General/LoginPublisherAccountByName",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("dexcom login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dexcom login: status %d", resp.StatusCode)
	}
	var sessionID string
	if err := json.NewDecoder(resp.Body).Decode(&sessionID); err != nil {
		return fmt.Errorf("dexcom login: decode session: %w", err)
	}
	c.sessionID = sessionID
	return nil
}

func (c *Client) FetchLatest(account string) (*types.Reading, error) {
	if c.sessionID == "" {
		if err := c.Login(); err != nil {
			return nil, err
		}
	}
	reading, err := c.fetchLatestRaw(account)
	if err != nil && strings.Contains(err.Error(), "session expired") {
		if loginErr := c.Login(); loginErr != nil {
			return nil, loginErr
		}
		return c.fetchLatestRaw(account)
	}
	return reading, err
}

type dexcomReading struct {
	WT    string `json:"WT"`
	Value int    `json:"Value"`
	Trend string `json:"Trend"`
}

var dateRe = regexp.MustCompile(`/Date\((\d+)`)

func (c *Client) fetchLatestRaw(account string) (*types.Reading, error) {
	url := fmt.Sprintf("%s/Publisher/ReadPublisherLatestGlucoseValues?sessionId=%s&minutes=10&maxCount=1",
		c.base, c.sessionID)
	resp, err := c.http.Post(url, "application/json", strings.NewReader(""))
	if err != nil {
		return nil, fmt.Errorf("dexcom fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusInternalServerError {
		c.sessionID = ""
		return nil, fmt.Errorf("dexcom fetch: session expired")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dexcom fetch: status %d", resp.StatusCode)
	}
	var readings []dexcomReading
	if err := json.NewDecoder(resp.Body).Decode(&readings); err != nil {
		return nil, fmt.Errorf("dexcom fetch: decode: %w", err)
	}
	if len(readings) == 0 {
		return nil, nil
	}
	r := readings[0]
	ms, err := parseWallTime(r.WT)
	if err != nil {
		return nil, fmt.Errorf("dexcom fetch: parse time: %w", err)
	}
	return &types.Reading{
		Account:    account,
		Value:      r.Value,
		Trend:      mapTrend(r.Trend),
		RecordedAt: time.UnixMilli(ms).UTC(),
	}, nil
}

func parseWallTime(wt string) (int64, error) {
	m := dateRe.FindStringSubmatch(wt)
	if len(m) < 2 {
		return 0, fmt.Errorf("unexpected WT format: %s", wt)
	}
	return strconv.ParseInt(m[1], 10, 64)
}

func mapTrend(t string) types.Trend {
	switch t {
	case "DoubleUp":
		return types.TrendDoubleUp
	case "SingleUp":
		return types.TrendSingleUp
	case "FortyFiveUp":
		return types.TrendFortyFiveUp
	case "Flat":
		return types.TrendFlat
	case "FortyFiveDown":
		return types.TrendFortyFiveDown
	case "SingleDown":
		return types.TrendSingleDown
	case "DoubleDown":
		return types.TrendDoubleDown
	case "NotComputable":
		return types.TrendNotComputable
	case "RateOutOfRange":
		return types.TrendRateOutOfRange
	default:
		return types.TrendNone
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./dexcom/... -v
```

Expected: all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add dexcom/
git commit -m "feat: add Dexcom Share API client with transparent re-auth"
```

---

### Task 8: Evaluator

**Files:**
- Create: `evaluator/evaluator.go`
- Create: `evaluator/evaluator_test.go`

- [ ] **Step 1: Write failing tests**

```go
// evaluator/evaluator_test.go
package evaluator_test

import (
	"testing"
	"time"

	"dexmon/config"
	"dexmon/evaluator"
	"dexmon/types"
)

// mockStore is an in-memory AlarmStateReader for tests.
type mockStore struct {
	states map[string]*types.AlarmState
}

func newMockStore() *mockStore { return &mockStore{states: map[string]*types.AlarmState{}} }

func (m *mockStore) key(account, alarm, recipient string) string {
	return account + "/" + alarm + "/" + recipient
}

func (m *mockStore) GetAlarmState(account, alarm, recipient string) (*types.AlarmState, error) {
	if s, ok := m.states[m.key(account, alarm, recipient)]; ok {
		return s, nil
	}
	return &types.AlarmState{Account: account, AlarmName: alarm, Recipient: recipient}, nil
}

func (m *mockStore) set(account, alarm, recipient string, state *types.AlarmState) {
	m.states[m.key(account, alarm, recipient)] = state
}

var baseAlarm = config.AlarmConfig{
	Name:            "Low",
	Threshold:       70,
	Direction:       "below",
	Trend:           []string{"flat", "forty_five_down", "single_down", "double_down"},
	Priority:        "high",
	Backoff:         "30m",
	RearmOnRecovery: true,
	Recipients:      []string{"brandon"},
}

func TestEvaluate_FiresWhenTriggered(t *testing.T) {
	store := newMockStore()
	reading := types.Reading{Account: "jessica", Value: 65, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, rearm, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fire) != 1 {
		t.Fatalf("expected 1 fire result, got %d", len(fire))
	}
	if fire[0].Recipient != "brandon" {
		t.Errorf("unexpected recipient: %s", fire[0].Recipient)
	}
	if len(rearm) != 0 {
		t.Errorf("expected 0 rearm, got %d", len(rearm))
	}
}

func TestEvaluate_DoesNotFireWhenAboveThreshold(t *testing.T) {
	store := newMockStore()
	reading := types.Reading{Account: "jessica", Value: 75, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire for in-range reading, got %d", len(fire))
	}
}

func TestEvaluate_DoesNotFireWhenTrendNotMatching(t *testing.T) {
	store := newMockStore()
	// Value is below threshold but trend is rising (single_up), not in filter list
	reading := types.Reading{Account: "jessica", Value: 65, Trend: types.TrendSingleUp}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire for non-matching trend, got %d", len(fire))
	}
}

func TestEvaluate_SkipsWhenSnoozed(t *testing.T) {
	store := newMockStore()
	future := time.Now().UTC().Add(30 * time.Minute)
	store.set("jessica", "Low", "brandon", &types.AlarmState{SnoozedUntil: &future})

	reading := types.Reading{Account: "jessica", Value: 65, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire during snooze, got %d", len(fire))
	}
}

func TestEvaluate_SkipsWhenWithinBackoff(t *testing.T) {
	store := newMockStore()
	recent := time.Now().UTC().Add(-5 * time.Minute) // fired 5m ago, backoff is 30m
	store.set("jessica", "Low", "brandon", &types.AlarmState{LastFiredAt: &recent})

	reading := types.Reading{Account: "jessica", Value: 65, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire within backoff, got %d", len(fire))
	}
}

func TestEvaluate_FiresAfterBackoffExpires(t *testing.T) {
	store := newMockStore()
	longAgo := time.Now().UTC().Add(-60 * time.Minute) // 60m ago, backoff is 30m
	store.set("jessica", "Low", "brandon", &types.AlarmState{LastFiredAt: &longAgo})

	reading := types.Reading{Account: "jessica", Value: 65, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 1 {
		t.Errorf("expected 1 fire after backoff, got %d", len(fire))
	}
}

func TestEvaluate_SkipsWhenActiveEmergencyReceipt(t *testing.T) {
	store := newMockStore()
	rid := "receipt-xyz"
	expires := time.Now().UTC().Add(1 * time.Hour) // receipt still active
	store.set("jessica", "Severe Low", "brandon", &types.AlarmState{ReceiptID: &rid, ReceiptExpiresAt: &expires})

	emergencyAlarm := config.AlarmConfig{
		Name:       "Severe Low",
		Threshold:  55,
		Direction:  "below",
		Trend:      []string{"flat"},
		Priority:   "emergency",
		Retry:      "5m",
		Expire:     "2h",
		Recipients: []string{"brandon"},
	}
	reading := types.Reading{Account: "jessica", Value: 50, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{emergencyAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire with active emergency receipt, got %d", len(fire))
	}
}

func TestEvaluate_FiresWhenEmergencyReceiptExpired(t *testing.T) {
	store := newMockStore()
	rid := "receipt-old"
	expired := time.Now().UTC().Add(-1 * time.Minute) // expired
	store.set("jessica", "Severe Low", "brandon", &types.AlarmState{ReceiptID: &rid, ReceiptExpiresAt: &expired})

	emergencyAlarm := config.AlarmConfig{
		Name:       "Severe Low",
		Threshold:  55,
		Direction:  "below",
		Trend:      []string{"flat"},
		Priority:   "emergency",
		Retry:      "5m",
		Expire:     "2h",
		Recipients: []string{"brandon"},
	}
	reading := types.Reading{Account: "jessica", Value: 50, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{emergencyAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 1 {
		t.Errorf("expected 1 fire for expired receipt, got %d", len(fire))
	}
}

func TestEvaluate_RearmsOnRecovery(t *testing.T) {
	store := newMockStore()
	recent := time.Now().UTC().Add(-5 * time.Minute)
	store.set("jessica", "Low", "brandon", &types.AlarmState{LastFiredAt: &recent})

	// Reading is now above threshold (recovered)
	reading := types.Reading{Account: "jessica", Value: 80, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, rearm, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire on recovery, got %d", len(fire))
	}
	if len(rearm) != 1 {
		t.Errorf("expected 1 rearm on recovery, got %d", len(rearm))
	}
}

func TestEvaluate_NoRearmWhenDisabled(t *testing.T) {
	store := newMockStore()
	recent := time.Now().UTC().Add(-5 * time.Minute)
	store.set("jessica", "High", "brandon", &types.AlarmState{LastFiredAt: &recent})

	noRearmAlarm := config.AlarmConfig{
		Name:            "High",
		Threshold:       250,
		Direction:       "above",
		Trend:           []string{"flat"},
		Priority:        "normal",
		Backoff:         "60m",
		RearmOnRecovery: false,
		Recipients:      []string{"brandon"},
	}
	// Reading is now below threshold (recovered)
	reading := types.Reading{Account: "jessica", Value: 200, Trend: types.TrendFlat}
	now := time.Now().UTC()

	_, rearm, err := evaluator.Evaluate("jessica", []config.AlarmConfig{noRearmAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rearm) != 0 {
		t.Errorf("expected no rearm when rearm_on_recovery=false, got %d", len(rearm))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./evaluator/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write evaluator/evaluator.go**

```go
package evaluator

import (
	"time"

	"dexmon/config"
	"dexmon/types"
)

// AlarmStateReader is the store interface Evaluate needs — allows test mocks.
type AlarmStateReader interface {
	GetAlarmState(account, alarmName, recipient string) (*types.AlarmState, error)
}

// EvalResult is a (alarm, recipient) pair that should be acted upon.
type EvalResult struct {
	AlarmName string
	Recipient string
	Alarm     config.AlarmConfig
}

// Evaluate checks a reading against all alarms and returns:
//   - toFire: recipients that should receive a notification now
//   - toRearm: recipients whose alarm state should be cleared (rearm_on_recovery)
func Evaluate(account string, alarms []config.AlarmConfig, reading types.Reading, store AlarmStateReader, now time.Time) (toFire []EvalResult, toRearm []EvalResult, err error) {
	for _, alarm := range alarms {
		triggered := isTriggered(reading.Value, alarm.Threshold, alarm.Direction) &&
			trendMatches(reading.Trend, alarm.Trend)

		for _, recipient := range alarm.Recipients {
			state, err := store.GetAlarmState(account, alarm.Name, recipient)
			if err != nil {
				return nil, nil, err
			}

			if triggered {
				if shouldFire(alarm, state, now) {
					toFire = append(toFire, EvalResult{
						AlarmName: alarm.Name,
						Recipient: recipient,
						Alarm:     alarm,
					})
				}
			} else if alarm.RearmOnRecovery && (state.LastFiredAt != nil || state.SnoozedUntil != nil) {
				toRearm = append(toRearm, EvalResult{
					AlarmName: alarm.Name,
					Recipient: recipient,
					Alarm:     alarm,
				})
			}
		}
	}
	return toFire, toRearm, nil
}

func isTriggered(value, threshold int, direction string) bool {
	switch direction {
	case "above":
		return value > threshold
	case "below":
		return value < threshold
	default:
		return false
	}
}

func trendMatches(trend types.Trend, allowed []string) bool {
	for _, t := range allowed {
		if string(trend) == t {
			return true
		}
	}
	return false
}

func shouldFire(alarm config.AlarmConfig, state *types.AlarmState, now time.Time) bool {
	if state.SnoozedUntil != nil && now.Before(*state.SnoozedUntil) {
		return false
	}
	if state.ReceiptID != nil && state.ReceiptExpiresAt != nil && now.Before(*state.ReceiptExpiresAt) {
		return false
	}
	if alarm.Priority != "emergency" && alarm.Backoff != "" && state.LastFiredAt != nil {
		backoff, err := time.ParseDuration(alarm.Backoff)
		if err == nil && now.Sub(*state.LastFiredAt) < backoff {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./evaluator/... -v
```

Expected: all 9 tests pass.

- [ ] **Step 5: Commit**

```bash
git add evaluator/
git commit -m "feat: add alarm evaluator with threshold, trend, backoff, snooze, and receipt checks"
```

---

### Task 9: Dispatcher

**Files:**
- Create: `dispatcher/dispatcher.go`
- Create: `dispatcher/dispatcher_test.go`

- [ ] **Step 1: Write failing tests**

```go
// dispatcher/dispatcher_test.go
package dispatcher_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
	"dexmon/store"
	"dexmon/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func parsePushoverForm(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	body, _ := io.ReadAll(r.Body)
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("parse form: %v", err)
	}
	return vals
}

func TestSend_NormalPriority(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = parsePushoverForm(t, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")

	alarm := config.AlarmConfig{Name: "High", Priority: "normal", Backoff: "60m"}
	err := d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "High",
		Recipient: "brandon",
		UserKey:   "user-key-brandon",
		Message:   "High: BG 260 ↗",
		Alarm:     alarm,
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if captured.Get("priority") != "0" {
		t.Errorf("priority: got %q, want %q", captured.Get("priority"), "0")
	}
	if captured.Get("user") != "user-key-brandon" {
		t.Errorf("user key not forwarded")
	}
}

func TestSend_HighPriority(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = parsePushoverForm(t, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")

	alarm := config.AlarmConfig{Name: "Low", Priority: "high", Backoff: "30m"}
	_ = d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Message:   "Low: BG 65 ↘",
		Alarm:     alarm,
	}, time.Now().UTC())

	if captured.Get("priority") != "1" {
		t.Errorf("priority: got %q, want %q", captured.Get("priority"), "1")
	}
}

func TestSend_EmergencyPriority_SetsRetryExpireCallback(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = parsePushoverForm(t, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1, "receipt": "receipt-abc"})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "https://example.com/pushover/callback")

	alarm := config.AlarmConfig{
		Name:     "Severe Low",
		Priority: "emergency",
		Retry:    "5m",
		Expire:   "2h",
	}
	now := time.Now().UTC()
	err := d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Severe Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Message:   "Severe Low: BG 50 ↓↓",
		Alarm:     alarm,
	}, now)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if captured.Get("priority") != "2" {
		t.Errorf("priority: got %q, want 2", captured.Get("priority"))
	}
	if captured.Get("retry") != "300" {
		t.Errorf("retry: got %q, want 300", captured.Get("retry"))
	}
	if captured.Get("expire") != "7200" {
		t.Errorf("expire: got %q, want 7200", captured.Get("expire"))
	}
	if !strings.Contains(captured.Get("callback"), "pushover/callback") {
		t.Errorf("callback URL not set: %q", captured.Get("callback"))
	}

	// Verify alarm state written to store
	state, err := st.GetAlarmState("jessica", "Severe Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if state.LastFiredAt == nil {
		t.Error("expected LastFiredAt set")
	}
	if state.ReceiptID == nil || *state.ReceiptID != "receipt-abc" {
		t.Errorf("expected ReceiptID 'receipt-abc', got %v", state.ReceiptID)
	}
	if state.ReceiptExpiresAt == nil {
		t.Error("expected ReceiptExpiresAt set")
	} else {
		wantExpiry := now.Add(2 * time.Hour)
		if state.ReceiptExpiresAt.Before(wantExpiry.Add(-2*time.Second)) ||
			state.ReceiptExpiresAt.After(wantExpiry.Add(2*time.Second)) {
			t.Errorf("ReceiptExpiresAt: got %v, want ~%v", *state.ReceiptExpiresAt, wantExpiry)
		}
	}
}

func TestSend_LeavesStateUntouchedOnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")

	alarm := config.AlarmConfig{Name: "Low", Priority: "high"}
	err := d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Alarm:     alarm,
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for 429 response")
	}

	state, _ := st.GetAlarmState("jessica", "Low", "brandon")
	if state.LastFiredAt != nil {
		t.Error("alarm state should not be updated on API error")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./dispatcher/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write dispatcher/dispatcher.go**

```go
package dispatcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dexmon/config"
	"dexmon/store"
	"dexmon/types"
)

const defaultPushoverAPI = "https://api.pushover.net/1/messages.json"

type Dispatcher struct {
	apiURL      string
	appToken    string
	store       *store.Store
	callbackURL string
	http        *http.Client
}

func New(appToken string, store *store.Store, callbackURL string) *Dispatcher {
	return NewWithAPI(defaultPushoverAPI, appToken, store, callbackURL)
}

func NewWithAPI(apiURL, appToken string, store *store.Store, callbackURL string) *Dispatcher {
	return &Dispatcher{
		apiURL:      apiURL,
		appToken:    appToken,
		store:       store,
		callbackURL: callbackURL,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

type SendRequest struct {
	Account   string
	AlarmName string
	Recipient string
	UserKey   string
	Message   string
	Alarm     config.AlarmConfig
}

func (d *Dispatcher) Send(req SendRequest, now time.Time) error {
	priority := priorityCode(req.Alarm.Priority)

	form := url.Values{
		"token":    {d.appToken},
		"user":     {req.UserKey},
		"message":  {req.Message},
		"priority": {fmt.Sprintf("%d", priority)},
	}

	if req.Alarm.Priority == "emergency" {
		form.Set("retry", durationSeconds(req.Alarm.Retry))
		form.Set("expire", durationSeconds(req.Alarm.Expire))
		if d.callbackURL != "" {
			form.Set("callback", d.callbackURL)
		}
	}

	resp, err := d.http.Post(d.apiURL, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("pushover send: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status  int    `json:"status"`
		Receipt string `json:"receipt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("pushover send: decode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover send: status %d", resp.StatusCode)
	}

	lastFired := now
	state := types.AlarmState{
		Account:     req.Account,
		AlarmName:   req.AlarmName,
		Recipient:   req.Recipient,
		LastFiredAt: &lastFired,
	}

	if req.Alarm.Priority == "emergency" && result.Receipt != "" {
		rid := result.Receipt
		state.ReceiptID = &rid
		expireDur, _ := time.ParseDuration(req.Alarm.Expire)
		t := now.Add(expireDur)
		state.ReceiptExpiresAt = &t
	}

	return d.store.UpsertAlarmState(state)
}

func priorityCode(p string) int {
	switch p {
	case "emergency":
		return 2
	case "high":
		return 1
	default:
		return 0
	}
}

func durationSeconds(s string) string {
	d, _ := time.ParseDuration(s)
	return fmt.Sprintf("%d", int(d.Seconds()))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./dispatcher/... -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add dispatcher/
git commit -m "feat: add Pushover dispatcher with emergency receipt tracking"
```

---

### Task 10: Health Monitoring

**Files:**
- Create: `health/health.go`

- [ ] **Step 1: Write health/health.go**

```go
package health

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func PingWatchdog(url string) {
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Printf("watchdog ping failed: %v", err)
		return
	}
	resp.Body.Close()
}

func FireMissedReadingsAlarm(account string, disp *dispatcher.Dispatcher, recipients map[string]config.RecipientConfig, healthCfg config.HealthConfig) {
	alarm := config.AlarmConfig{
		Name:     "Dexcom Unreachable",
		Priority: healthCfg.DexcomTimeout.Priority,
		Retry:    "5m",
		Expire:   "2h",
	}
	for _, recipientName := range healthCfg.DexcomTimeout.Recipients {
		recipientCfg, ok := recipients[recipientName]
		if !ok {
			log.Printf("health alarm: unknown recipient %q", recipientName)
			continue
		}
		req := dispatcher.SendRequest{
			Account:   account,
			AlarmName: alarm.Name,
			Recipient: recipientName,
			UserKey:   recipientCfg.PushoverUserKey,
			Message:   fmt.Sprintf("Dexcom unreachable for account %s", account),
			Alarm:     alarm,
		}
		if err := disp.Send(req, time.Now().UTC()); err != nil {
			log.Printf("health alarm dispatch to %s: %v", recipientName, err)
		}
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./health/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add health/
git commit -m "feat: add health monitoring (watchdog ping, missed readings alarm)"
```

---

### Task 11: Poller

**Files:**
- Create: `poller/poller.go`
- Create: `poller/poller_test.go`

- [ ] **Step 1: Write failing tests**

```go
// poller/poller_test.go
package poller_test

import (
	"testing"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
	"dexmon/poller"
	"dexmon/store"
	"dexmon/types"
)

// mockFetcher simulates a Dexcom client for tests.
type mockFetcher struct {
	readings []*types.Reading
	idx      int
	err      error
}

func (m *mockFetcher) Login() error { return nil }

func (m *mockFetcher) FetchLatest(account string) (*types.Reading, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idx >= len(m.readings) {
		return nil, nil
	}
	r := m.readings[m.idx]
	m.idx++
	return r, nil
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPoller_DeduplicatesReadings(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Second)
	reading := &types.Reading{Account: "jessica", Value: 85, Trend: types.TrendFlat, RecordedAt: ts}

	// Return the same reading twice
	fetcher := &mockFetcher{readings: []*types.Reading{reading, reading}}

	st := newTestStore(t)

	// Pushover calls should not happen; use a server that fails if called
	// No alarm configured, so no dispatching will happen
	disp := dispatcher.NewWithAPI("http://127.0.0.1:0", "tok", st, "")

	cfg := config.AccountConfig{
		DexcomUsername: "u",
		DexcomPassword: "p",
		PollInterval:   "5m",
		Alarms:         []config.AlarmConfig{},
	}
	healthCfg := config.HealthConfig{
		DexcomTimeout: config.DexcomTimeoutConfig{MaxMissedReadings: 3},
	}

	p := poller.New("jessica", cfg, fetcher, st, disp, map[string]config.RecipientConfig{}, healthCfg)

	p.Tick() // first tick — inserts reading
	p.Tick() // second tick — deduplicates (same recorded_at)

	// Exactly one reading should be in the store
	count, err := st.CountReadings("jessica")
	if err != nil {
		t.Fatalf("CountReadings: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 reading after deduplication, got %d", count)
	}
}

func TestPoller_FiresHealthAlarmAfterMaxMisses(t *testing.T) {
	import_errors := 0
	pushoverSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		import_errors++
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer pushoverSrv.Close()

	fetcher := &mockFetcher{err: fmt.Errorf("network error")}
	st := newTestStore(t)
	disp := dispatcher.NewWithAPI(pushoverSrv.URL, "tok", st, "")

	cfg := config.AccountConfig{PollInterval: "5m", Alarms: []config.AlarmConfig{}}
	healthCfg := config.HealthConfig{
		DexcomTimeout: config.DexcomTimeoutConfig{
			MaxMissedReadings: 3,
			Priority:          "high",
			Recipients:        []string{"brandon"},
		},
	}
	recipients := map[string]config.RecipientConfig{
		"brandon": {PushoverUserKey: "ukey"},
	}

	p := poller.New("jessica", cfg, fetcher, st, disp, recipients, healthCfg)

	p.Tick() // miss 1
	p.Tick() // miss 2
	p.Tick() // miss 3 — should fire health alarm

	if import_errors != 1 {
		t.Errorf("expected 1 health alarm dispatch, got %d", import_errors)
	}

	p.Tick() // miss 4 — should NOT fire again (already fired)

	if import_errors != 1 {
		t.Errorf("expected health alarm to fire only once, got %d", import_errors)
	}
}
```

Note: `TestPoller_FiresHealthAlarmAfterMaxMisses` also needs `net/http/httptest`, `encoding/json`, and `fmt` imports. Adjust the import block in the test file.

Also, the test requires a `store.CountReadings` method. Add it to the store before running the test.

- [ ] **Step 2: Add CountReadings to store/readings.go**

Append to `store/readings.go`:

```go
func (s *Store) CountReadings(account string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM readings WHERE account = ?`, account).Scan(&count)
	return count, err
}
```

- [ ] **Step 3: Write poller/poller_test.go (corrected import block)**

The full file with all imports:

```go
// poller/poller_test.go
package poller_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
	"dexmon/poller"
	"dexmon/store"
	"dexmon/types"
)

type mockFetcher struct {
	readings []*types.Reading
	idx      int
	err      error
}

func (m *mockFetcher) Login() error { return nil }

func (m *mockFetcher) FetchLatest(account string) (*types.Reading, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idx >= len(m.readings) {
		return nil, nil
	}
	r := m.readings[m.idx]
	m.idx++
	return r, nil
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPoller_DeduplicatesReadings(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Second)
	reading := &types.Reading{Account: "jessica", Value: 85, Trend: types.TrendFlat, RecordedAt: ts}
	fetcher := &mockFetcher{readings: []*types.Reading{reading, reading}}

	st := newTestStore(t)
	disp := dispatcher.NewWithAPI("http://127.0.0.1:0", "tok", st, "")

	cfg := config.AccountConfig{
		DexcomUsername: "u",
		DexcomPassword: "p",
		PollInterval:   "5m",
		Alarms:         []config.AlarmConfig{},
	}
	healthCfg := config.HealthConfig{
		DexcomTimeout: config.DexcomTimeoutConfig{MaxMissedReadings: 3},
	}

	p := poller.New("jessica", cfg, fetcher, st, disp, map[string]config.RecipientConfig{}, healthCfg)
	p.Tick()
	p.Tick()

	count, err := st.CountReadings("jessica")
	if err != nil {
		t.Fatalf("CountReadings: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 reading after deduplication, got %d", count)
	}
}

func TestPoller_FiresHealthAlarmAfterMaxMisses(t *testing.T) {
	callCount := 0
	pushoverSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer pushoverSrv.Close()

	fetcher := &mockFetcher{err: fmt.Errorf("network error")}
	st := newTestStore(t)
	disp := dispatcher.NewWithAPI(pushoverSrv.URL, "tok", st, "")

	cfg := config.AccountConfig{PollInterval: "5m", Alarms: []config.AlarmConfig{}}
	healthCfg := config.HealthConfig{
		DexcomTimeout: config.DexcomTimeoutConfig{
			MaxMissedReadings: 3,
			Priority:          "high",
			Recipients:        []string{"brandon"},
		},
	}
	recipients := map[string]config.RecipientConfig{
		"brandon": {PushoverUserKey: "ukey"},
	}

	p := poller.New("jessica", cfg, fetcher, st, disp, recipients, healthCfg)
	p.Tick() // miss 1
	p.Tick() // miss 2
	p.Tick() // miss 3 — fires health alarm

	if callCount != 1 {
		t.Errorf("expected 1 health alarm dispatch, got %d", callCount)
	}

	p.Tick() // miss 4 — should NOT re-fire
	if callCount != 1 {
		t.Errorf("expected health alarm to fire only once, got %d", callCount)
	}
}
```

- [ ] **Step 4: Run tests to confirm they fail**

```bash
go test ./poller/... -v
```

Expected: compilation error (`poller.New`, `p.Tick` undefined; `Fetcher` interface not declared).

- [ ] **Step 5: Write poller/poller.go**

```go
package poller

import (
	"fmt"
	"log"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
	"dexmon/evaluator"
	"dexmon/health"
	"dexmon/store"
	"dexmon/types"
)

// Fetcher is satisfied by *dexcom.Client and test mocks.
type Fetcher interface {
	Login() error
	FetchLatest(account string) (*types.Reading, error)
}

type Poller struct {
	accountName      string
	cfg              config.AccountConfig
	fetcher          Fetcher
	store            *store.Store
	disp             *dispatcher.Dispatcher
	recipients       map[string]config.RecipientConfig
	healthCfg        config.HealthConfig
	missCount        int
	healthAlarmFired bool
}

func New(accountName string, cfg config.AccountConfig, fetcher Fetcher, st *store.Store, disp *dispatcher.Dispatcher, recipients map[string]config.RecipientConfig, healthCfg config.HealthConfig) *Poller {
	return &Poller{
		accountName: accountName,
		cfg:         cfg,
		fetcher:     fetcher,
		store:       st,
		disp:        disp,
		recipients:  recipients,
		healthCfg:   healthCfg,
	}
}

func (p *Poller) Run() {
	if err := p.fetcher.Login(); err != nil {
		log.Printf("[%s] initial login failed: %v", p.accountName, err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -30)
	if err := p.store.PruneReadings(p.accountName, cutoff); err != nil {
		log.Printf("[%s] prune readings: %v", p.accountName, err)
	}
	interval, _ := time.ParseDuration(p.cfg.PollInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		p.Tick()
	}
}

// Tick executes one poll cycle. Exported for testing.
func (p *Poller) Tick() {
	reading, err := p.fetcher.FetchLatest(p.accountName)
	if err != nil {
		p.missCount++
		log.Printf("[%s] fetch error (%d consecutive): %v", p.accountName, p.missCount, err)
		if p.missCount >= p.healthCfg.DexcomTimeout.MaxMissedReadings && !p.healthAlarmFired {
			health.FireMissedReadingsAlarm(p.accountName, p.disp, p.recipients, p.healthCfg)
			p.healthAlarmFired = true
		}
		return
	}
	if reading == nil {
		return
	}

	exists, err := p.store.HasReading(p.accountName, reading.RecordedAt)
	if err != nil {
		log.Printf("[%s] check reading: %v", p.accountName, err)
		return
	}
	if exists {
		return
	}

	if err := p.store.InsertReading(*reading); err != nil {
		log.Fatalf("[%s] insert reading (store fatal): %v", p.accountName, err)
	}

	p.missCount = 0
	p.healthAlarmFired = false

	if url := p.healthCfg.Watchdog.PingURL; url != "" {
		health.PingWatchdog(url)
	}

	toFire, toRearm, err := evaluator.Evaluate(p.accountName, p.cfg.Alarms, *reading, p.store, time.Now().UTC())
	if err != nil {
		log.Printf("[%s] evaluate: %v", p.accountName, err)
		return
	}

	for _, result := range toRearm {
		if err := p.store.ClearAlarmRearm(p.accountName, result.AlarmName, result.Recipient); err != nil {
			log.Printf("[%s] clear alarm rearm: %v", p.accountName, err)
		}
	}

	for _, result := range toFire {
		recipientCfg := p.recipients[result.Recipient]
		req := dispatcher.SendRequest{
			Account:   p.accountName,
			AlarmName: result.AlarmName,
			Recipient: result.Recipient,
			UserKey:   recipientCfg.PushoverUserKey,
			Message:   formatMessage(*reading, result.Alarm),
			Alarm:     result.Alarm,
		}
		if err := p.disp.Send(req, time.Now().UTC()); err != nil {
			log.Printf("[%s] dispatch to %s: %v", p.accountName, result.Recipient, err)
		}
	}
}

func formatMessage(r types.Reading, alarm config.AlarmConfig) string {
	return fmt.Sprintf("%s: BG %d %s", alarm.Name, r.Value, trendArrow(r.Trend))
}

func trendArrow(t types.Trend) string {
	switch t {
	case types.TrendDoubleUp:
		return "↑↑"
	case types.TrendSingleUp:
		return "↑"
	case types.TrendFortyFiveUp:
		return "↗"
	case types.TrendFlat:
		return "→"
	case types.TrendFortyFiveDown:
		return "↘"
	case types.TrendSingleDown:
		return "↓"
	case types.TrendDoubleDown:
		return "↓↓"
	default:
		return ""
	}
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./poller/... -v
```

Expected: both tests pass.

- [ ] **Step 7: Commit**

```bash
git add store/readings.go poller/
git commit -m "feat: add poller with deduplication and health alarm throttle"
```

---

### Task 12: Callback Server

**Files:**
- Create: `callback/server.go`
- Create: `callback/server_test.go`

- [ ] **Step 1: Write failing tests**

```go
// callback/server_test.go
package callback_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dexmon/callback"
	"dexmon/store"
	"dexmon/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCallback_ClearsReceiptOnAck(t *testing.T) {
	st := newTestStore(t)
	rid := "receipt-123"
	expires := time.Now().UTC().Add(2 * time.Hour)
	now := time.Now().UTC()
	_ = st.UpsertAlarmState(types.AlarmState{
		Account:          "jessica",
		AlarmName:        "Severe Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	})

	srv := callback.New(st, 0)
	body, _ := json.Marshal(map[string]interface{}{
		"receipt":         "receipt-123",
		"acknowledged_at": time.Now().Unix(),
		"snooze":          0,
	})
	req := httptest.NewRequest(http.MethodPost, "/pushover/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	state, _ := st.GetAlarmState("jessica", "Severe Low", "brandon")
	if state.ReceiptID != nil {
		t.Error("expected ReceiptID cleared on acknowledgment")
	}
}

func TestCallback_SetsSnoozedUntilWhenSnoozeProvided(t *testing.T) {
	st := newTestStore(t)
	rid := "receipt-456"
	expires := time.Now().UTC().Add(2 * time.Hour)
	now := time.Now().UTC()
	_ = st.UpsertAlarmState(types.AlarmState{
		Account:          "jessica",
		AlarmName:        "Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	})

	srv := callback.New(st, 0)
	body, _ := json.Marshal(map[string]interface{}{
		"receipt":         "receipt-456",
		"acknowledged_at": time.Now().Unix(),
		"snooze":          1800, // 30 minutes in seconds
	})
	req := httptest.NewRequest(http.MethodPost, "/pushover/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	state, _ := st.GetAlarmState("jessica", "Low", "brandon")
	if state.ReceiptID != nil {
		t.Error("expected ReceiptID cleared")
	}
	if state.SnoozedUntil == nil {
		t.Fatal("expected SnoozedUntil set")
	}
	minExpected := time.Now().UTC().Add(29 * time.Minute)
	if state.SnoozedUntil.Before(minExpected) {
		t.Errorf("SnoozedUntil %v is earlier than expected ~30m from now", *state.SnoozedUntil)
	}
}

func TestCallback_IgnoresUnknownReceipt(t *testing.T) {
	st := newTestStore(t)
	srv := callback.New(st, 0)

	body, _ := json.Marshal(map[string]interface{}{
		"receipt":         "unknown-receipt",
		"acknowledged_at": time.Now().Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/pushover/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown receipt, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./callback/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write callback/server.go**

```go
package callback

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"dexmon/store"
)

type Server struct {
	store *store.Store
	port  int
	mux   *http.ServeMux
}

func New(store *store.Store, port int) *Server {
	s := &Server{store: store, port: port, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /pushover/callback", s.handleCallback)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Start() {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("callback server listening on %s", addr)
	if err := http.ListenAndServe(addr, s.mux); err != nil {
		log.Fatalf("callback server: %v", err)
	}
}

type callbackPayload struct {
	Receipt        string `json:"receipt"`
	AcknowledgedAt int64  `json:"acknowledged_at"`
	Snooze         int    `json:"snooze"` // seconds; 0 means no snooze
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	var payload callbackPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	state, err := s.store.GetAlarmStateByReceiptID(payload.Receipt)
	if err != nil {
		log.Printf("callback: lookup receipt %s: %v", payload.Receipt, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if state == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	state.ReceiptID = nil
	state.ReceiptExpiresAt = nil

	if payload.Snooze > 0 {
		snoozedUntil := time.Now().UTC().Add(time.Duration(payload.Snooze) * time.Second)
		state.SnoozedUntil = &snoozedUntil
	}

	if err := s.store.UpsertAlarmState(*state); err != nil {
		log.Printf("callback: update state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./callback/... -v
```

Expected: all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add callback/
git commit -m "feat: add Pushover callback server for acknowledgment and snooze"
```

---

### Task 13: Main Entry Point

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write main.go**

```go
package main

import (
	"flag"
	"log"
	"os"

	"dexmon/callback"
	"dexmon/config"
	"dexmon/dexcom"
	"dexmon/dispatcher"
	"dexmon/poller"
	"dexmon/store"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	dbPath := flag.String("db", "dexmon.db", "path to SQLite database")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	appToken := os.Getenv("PUSHOVER_APP_TOKEN")
	if appToken == "" {
		log.Fatal("PUSHOVER_APP_TOKEN environment variable is required")
	}

	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	disp := dispatcher.New(appToken, st, cfg.Server.CallbackURL)

	for name, acctCfg := range cfg.Accounts {
		client := dexcom.New(acctCfg.DexcomUsername, acctCfg.DexcomPassword)
		p := poller.New(name, acctCfg, client, st, disp, cfg.Recipients, cfg.Health)
		go p.Run()
	}

	srv := callback.New(st, cfg.Server.CallbackPort)
	srv.Start() // blocks
}
```

- [ ] **Step 2: Build the binary**

```bash
go build -o dexmon .
```

Expected: `dexmon` binary created with no errors.

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add main entry point and wire all components"
```

---

### Task 14: Example Config and Deployment File

**Files:**
- Create: `config.toml.example`
- Create: `dexmon.service`

- [ ] **Step 1: Write config.toml.example**

```toml
[server]
callback_port = 8080
callback_url  = "https://your-domain.com/pushover/callback"

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = ["brandon"]

  [health.watchdog]
  ping_url = "${HEALTHCHECKS_PING_URL}"

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

- [ ] **Step 2: Write dexmon.service**

```ini
[Unit]
Description=dexmon CGM alarm daemon
After=network.target

[Service]
Type=simple
User=dexmon
WorkingDirectory=/opt/dexmon
ExecStart=/opt/dexmon/dexmon -config /opt/dexmon/config.toml -db /opt/dexmon/dexmon.db
EnvironmentFile=/opt/dexmon/secrets.env
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 3: Add a README note about secrets.env format**

Append a comment block at the top of `config.toml.example`:

```toml
# secrets.env (mode 600, root-only) — referenced by dexmon.service EnvironmentFile:
#
#   PUSHOVER_APP_TOKEN=...
#   PUSHOVER_USER_KEY_BRANDON=...
#   PUSHOVER_USER_KEY_SARAH=...
#   PUSHOVER_USER_KEY_JESSICA=...
#   DEXCOM_USER_JESSICA=...
#   DEXCOM_PASS_JESSICA=...
#   HEALTHCHECKS_PING_URL=...
```

- [ ] **Step 4: Commit**

```bash
git add config.toml.example dexmon.service
git commit -m "feat: add example config and systemd service unit"
```

---

## Self-Review Against Spec

**Spec coverage check:**

| Spec requirement | Task(s) |
|---|---|
| One goroutine per account | Task 13 (main.go `go p.Run()`) |
| Dexcom Share API auth + re-auth | Task 7 |
| Deduplication by `recorded_at` | Task 11 (Tick, HasReading) |
| 30-day reading prune on startup | Task 11 (Run) |
| Threshold + trend filter | Task 8 (Evaluate) |
| Snooze check | Task 8 (shouldFire) |
| Backoff check (non-emergency) | Task 8 (shouldFire) |
| Emergency receipt block + expiry | Task 8 (shouldFire) |
| Rearm on recovery | Task 8, Task 11 (ClearAlarmRearm) |
| Pushover priority mapping | Task 9 |
| Emergency retry/expire/callback | Task 9 |
| Receipt stored in alarm_state | Task 9 |
| Pushover failure → no state change | Task 9 (test: LeavesStateUntouchedOnAPIError) |
| Callback clears receipt | Task 12 |
| Callback sets snooze per-recipient | Task 12 |
| Missed readings health alarm | Task 10, Task 11 |
| Watchdog ping on success | Task 11 (Tick) |
| Health alarm fires only once per outage | Task 11 (healthAlarmFired flag) |
| TOML config + env var expansion | Task 3 |
| Startup config validation (fatal) | Task 3 (validate) |
| SQLite fatal on write failure | Task 11 (`log.Fatalf` on InsertReading) |
| ARM-compatible binary | Task 1 (`modernc.org/sqlite`) |
| systemd deployment | Task 14 |

**All spec sections covered. No placeholders in code. Types consistent across tasks.**
