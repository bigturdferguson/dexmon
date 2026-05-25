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

	fire, rearm, err := evaluator.Evaluate("jessica", []config.AlarmConfig{baseAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 0 {
		t.Errorf("expected no fire during snooze, got %d", len(fire))
	}
	if len(rearm) != 0 {
		t.Errorf("expected no rearm while triggered+snoozed, got %d", len(rearm))
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

func TestEvaluate_EmergencyIgnoresBackoff(t *testing.T) {
	store := newMockStore()
	recent := time.Now().UTC().Add(-1 * time.Minute) // fired 1m ago, but emergency ignores backoff
	store.set("jessica", "Severe Low", "brandon", &types.AlarmState{LastFiredAt: &recent})

	emergencyAlarm := config.AlarmConfig{
		Name:       "Severe Low",
		Threshold:  55,
		Direction:  "below",
		Trend:      []string{"flat"},
		Priority:   "emergency",
		Retry:      "5m",
		Expire:     "2h",
		Backoff:    "60m", // would block a non-emergency, but emergency ignores this
		Recipients: []string{"brandon"},
	}
	reading := types.Reading{Account: "jessica", Value: 50, Trend: types.TrendFlat}
	now := time.Now().UTC()

	fire, _, err := evaluator.Evaluate("jessica", []config.AlarmConfig{emergencyAlarm}, reading, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fire) != 1 {
		t.Errorf("expected emergency alarm to fire despite backoff state, got %d fire results", len(fire))
	}
}

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
