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
