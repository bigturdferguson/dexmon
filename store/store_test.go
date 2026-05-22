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
