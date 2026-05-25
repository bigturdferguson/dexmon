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

	if err := s.InsertReading(types.Reading{Account: "jessica", Value: 80, Trend: types.TrendFlat, RecordedAt: old}); err != nil {
		t.Fatalf("setup InsertReading: %v", err)
	}
	if err := s.InsertReading(types.Reading{Account: "jessica", Value: 90, Trend: types.TrendFlat, RecordedAt: recent}); err != nil {
		t.Fatalf("setup InsertReading: %v", err)
	}

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

	if err := s.UpsertAlarmState(types.AlarmState{Account: "jessica", AlarmName: "Low", Recipient: "brandon", LastFiredAt: &t1}); err != nil {
		t.Fatalf("setup UpsertAlarmState: %v", err)
	}
	if err := s.UpsertAlarmState(types.AlarmState{Account: "jessica", AlarmName: "Low", Recipient: "brandon", LastFiredAt: &t2}); err != nil {
		t.Fatalf("setup UpsertAlarmState: %v", err)
	}

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
	if err := s.UpsertAlarmState(state); err != nil {
		t.Fatalf("setup UpsertAlarmState: %v", err)
	}

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

func TestGetReadings_ReturnsEmptyForNoData(t *testing.T) {
	s := newTestStore(t)
	readings, err := s.GetReadings("noah", time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetReadings: %v", err)
	}
	if len(readings) != 0 {
		t.Errorf("expected 0 readings, got %d", len(readings))
	}
}

func TestGetReadings_ExcludesReadingsBeforeSince(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	inWindow := now.Add(-1 * time.Hour)
	outOfWindow := now.Add(-25 * time.Hour)

	if err := s.InsertReading(types.Reading{Account: "noah", Value: 100, Trend: types.TrendFlat, RecordedAt: inWindow}); err != nil {
		t.Fatalf("InsertReading: %v", err)
	}
	if err := s.InsertReading(types.Reading{Account: "noah", Value: 80, Trend: types.TrendSingleDown, RecordedAt: outOfWindow}); err != nil {
		t.Fatalf("InsertReading: %v", err)
	}

	readings, err := s.GetReadings("noah", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetReadings: %v", err)
	}
	if len(readings) != 1 {
		t.Fatalf("expected 1 reading in window, got %d", len(readings))
	}
	if readings[0].Value != 100 {
		t.Errorf("expected value 100, got %d", readings[0].Value)
	}
}

func TestGetReadings_OrderedAscending(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	for _, r := range []types.Reading{
		{Account: "noah", Value: 120, Trend: types.TrendFlat, RecordedAt: now.Add(-2 * time.Hour)},
		{Account: "noah", Value: 140, Trend: types.TrendFlat, RecordedAt: now.Add(-1 * time.Hour)},
		{Account: "noah", Value: 100, Trend: types.TrendFlat, RecordedAt: now.Add(-3 * time.Hour)},
	} {
		if err := s.InsertReading(r); err != nil {
			t.Fatalf("InsertReading: %v", err)
		}
	}

	readings, err := s.GetReadings("noah", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetReadings: %v", err)
	}
	if len(readings) != 3 {
		t.Fatalf("expected 3 readings, got %d", len(readings))
	}
	if readings[0].Value != 100 || readings[1].Value != 120 || readings[2].Value != 140 {
		t.Errorf("readings not in ascending order by time: values=%v", []int{readings[0].Value, readings[1].Value, readings[2].Value})
	}
}

func TestGetReadingStats_ReturnsZerosForNoData(t *testing.T) {
	s := newTestStore(t)
	min, max, avg, err := s.GetReadingStats("noah", time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetReadingStats: %v", err)
	}
	if min != 0 || max != 0 || avg != 0 {
		t.Errorf("expected all zeros, got min=%d max=%d avg=%d", min, max, avg)
	}
}

func TestGetReadingStats_ReturnsCorrectValues(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	for _, r := range []types.Reading{
		{Account: "noah", Value: 72,  Trend: types.TrendFlat, RecordedAt: now.Add(-3 * time.Hour)},
		{Account: "noah", Value: 187, Trend: types.TrendFlat, RecordedAt: now.Add(-2 * time.Hour)},
		{Account: "noah", Value: 129, Trend: types.TrendFlat, RecordedAt: now.Add(-1 * time.Hour)},
	} {
		if err := s.InsertReading(r); err != nil {
			t.Fatalf("InsertReading: %v", err)
		}
	}

	min, max, avg, err := s.GetReadingStats("noah", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetReadingStats: %v", err)
	}
	if min != 72 {
		t.Errorf("expected min=72, got %d", min)
	}
	if max != 187 {
		t.Errorf("expected max=187, got %d", max)
	}
	// (72+187+129)/3 = 388/3 = 129 integer
	if avg != 129 {
		t.Errorf("expected avg=129, got %d", avg)
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
