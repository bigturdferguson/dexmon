package dashboard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dexmon/config"
	"dexmon/dashboard"
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

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestDashboardAPI_EmptyData(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Account != "noah" {
		t.Errorf("expected account=noah, got %q", resp.Account)
	}
	if resp.Current != nil {
		t.Error("expected nil current for no data")
	}
	if resp.Previous != nil {
		t.Error("expected nil previous for no data")
	}
	if resp.Stats.High != 0 || resp.Stats.Low != 0 || resp.Stats.Avg != 0 {
		t.Errorf("expected zero stats, got %+v", resp.Stats)
	}
	if resp.Readings == nil {
		t.Error("expected non-nil readings slice (may be empty)")
	}
	if resp.Alarms == nil {
		t.Error("expected non-nil alarms slice (may be empty)")
	}
	if resp.AlarmHistory == nil {
		t.Error("expected non-nil alarm_history slice (may be empty)")
	}
}

func TestDashboardAPI_PopulatedData(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	readings := []types.Reading{
		{Account: "noah", Value: 100, Trend: types.TrendFlat,       RecordedAt: now.Add(-2 * time.Hour)},
		{Account: "noah", Value: 72,  Trend: types.TrendSingleDown, RecordedAt: now.Add(-1 * time.Hour)},
		{Account: "noah", Value: 140, Trend: types.TrendSingleUp,   RecordedAt: now.Add(-5 * time.Minute)},
	}
	for _, r := range readings {
		if err := s.InsertReading(r); err != nil {
			t.Fatalf("InsertReading: %v", err)
		}
	}

	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Current == nil || resp.Current.Value != 140 {
		t.Errorf("expected current=140, got %v", resp.Current)
	}
	if resp.Previous == nil || resp.Previous.Value != 72 {
		t.Errorf("expected previous=72, got %v", resp.Previous)
	}
	if resp.Stats.High != 140 {
		t.Errorf("expected high=140, got %d", resp.Stats.High)
	}
	if resp.Stats.Low != 72 {
		t.Errorf("expected low=72, got %d", resp.Stats.Low)
	}
	if len(resp.Readings) != 3 {
		t.Errorf("expected 3 readings, got %d", len(resp.Readings))
	}
}

func TestDashboardAPI_AlarmStatus_NeverFired(t *testing.T) {
	s := newTestStore(t)
	alarms := []config.AlarmConfig{{Name: "Low", Priority: "high", Recipients: []string{"brandon"}}}
	recipients := map[string]config.RecipientConfig{"brandon": {PushoverUserKey: "key"}}

	h := dashboard.New(s, "noah", alarms, recipients, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(resp.Alarms))
	}
	if resp.Alarms[0].Status != "never_fired" {
		t.Errorf("expected never_fired, got %q", resp.Alarms[0].Status)
	}
	if resp.Alarms[0].LastFiredAt != nil {
		t.Error("expected nil last_fired_at for never_fired")
	}
}

func TestDashboardAPI_AlarmStatus_Active(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	expires := now.Add(2 * time.Hour)
	rid := "receipt-active"
	_ = s.UpsertAlarmState(types.AlarmState{
		Account:          "noah",
		AlarmName:        "Urgent Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	})

	alarms := []config.AlarmConfig{{Name: "Urgent Low", Priority: "emergency", Recipients: []string{"brandon"}}}
	h := dashboard.New(s, "noah", alarms, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(resp.Alarms))
	}
	if resp.Alarms[0].Status != "active" {
		t.Errorf("expected active, got %q", resp.Alarms[0].Status)
	}
}

func TestDashboardAPI_AlarmStatus_SnoozedUntil(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	snooze := now.Add(30 * time.Minute)
	_ = s.UpsertAlarmState(types.AlarmState{
		Account:      "noah",
		AlarmName:    "High",
		Recipient:    "brandon",
		LastFiredAt:  &now,
		SnoozedUntil: &snooze,
	})

	alarms := []config.AlarmConfig{{Name: "High", Priority: "high", Recipients: []string{"brandon"}}}
	h := dashboard.New(s, "noah", alarms, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Alarms[0].Status != "snoozed_until" {
		t.Errorf("expected snoozed_until, got %q", resp.Alarms[0].Status)
	}
	if resp.Alarms[0].SnoozedUntil == nil {
		t.Error("expected non-nil snoozed_until timestamp")
	}
}

func TestDashboardAPI_AlarmStatus_Fired(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Add(-1 * time.Hour)
	_ = s.UpsertAlarmState(types.AlarmState{
		Account:     "noah",
		AlarmName:   "Low",
		Recipient:   "brandon",
		LastFiredAt: &now,
	})

	alarms := []config.AlarmConfig{{Name: "Low", Priority: "high", Recipients: []string{"brandon"}}}
	h := dashboard.New(s, "noah", alarms, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Alarms[0].Status != "fired" {
		t.Errorf("expected fired, got %q", resp.Alarms[0].Status)
	}
}

func TestDashboardAPI_MultiRecipient_MostConcerningStatusWins(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	expires := now.Add(2 * time.Hour)
	rid := "receipt-multi"

	// alice: active (most concerning)
	_ = s.UpsertAlarmState(types.AlarmState{
		Account: "noah", AlarmName: "Low", Recipient: "alice",
		LastFiredAt: &now, ReceiptID: &rid, ReceiptExpiresAt: &expires,
	})
	// brandon: just fired
	fired := now.Add(-10 * time.Minute)
	_ = s.UpsertAlarmState(types.AlarmState{
		Account: "noah", AlarmName: "Low", Recipient: "brandon",
		LastFiredAt: &fired,
	})

	alarms := []config.AlarmConfig{
		{Name: "Low", Priority: "emergency", Recipients: []string{"alice", "brandon"}},
	}
	h := dashboard.New(s, "noah", alarms, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Alarms[0].Status != "active" {
		t.Errorf("expected active (most concerning), got %q", resp.Alarms[0].Status)
	}
}

func TestDashboardIndex_ServesHTML(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content-type, got %q", ct)
	}
}

func TestDashboardAPI_TargetRange(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 80, 140)
	w := get(t, h, "/api/dashboard")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Target.Low != 80 {
		t.Errorf("expected Target.Low=80, got %d", resp.Target.Low)
	}
	if resp.Target.High != 140 {
		t.Errorf("expected Target.High=140, got %d", resp.Target.High)
	}
}

func TestDashboardAPI_WindowDefault(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Window != "24h" {
		t.Errorf("expected window=24h, got %q", resp.Window)
	}
}

func TestDashboardAPI_Window7d(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard?window=7d")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Window != "7d" {
		t.Errorf("expected window=7d, got %q", resp.Window)
	}
}

func TestDashboardAPI_WindowInvalidFallsBack(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard?window=bogus")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Window != "24h" {
		t.Errorf("expected window=24h for invalid input, got %q", resp.Window)
	}
}

func TestDashboardAPI_Window12h(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard?window=12h")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Window != "12h" {
		t.Errorf("expected window=12h, got %q", resp.Window)
	}
}

func TestDashboardAPI_Window30d(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard?window=30d")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Window != "30d" {
		t.Errorf("expected window=30d, got %q", resp.Window)
	}
}

func TestDashboardAPI_Window12h_FiltersOldReadings(t *testing.T) {
	s := newTestStore(t)

	// Insert a reading 25 hours ago (outside 12h window)
	if err := s.InsertReading(types.Reading{
		Account:    "noah",
		Value:      200,
		RecordedAt: time.Now().UTC().Add(-25 * time.Hour),
		Trend:      types.TrendFlat,
	}); err != nil {
		t.Fatalf("save old reading: %v", err)
	}

	// Insert a reading 1 hour ago (inside 12h window)
	if err := s.InsertReading(types.Reading{
		Account:    "noah",
		Value:      120,
		RecordedAt: time.Now().UTC().Add(-1 * time.Hour),
		Trend:      types.TrendFlat,
	}); err != nil {
		t.Fatalf("save recent reading: %v", err)
	}

	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard?window=12h")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// The 25h-old reading (value 200) must not appear in the windowed response
	for _, r := range resp.Readings {
		if r.Value == 200 {
			t.Errorf("old reading (200, 25h ago) should be filtered out by 12h window, but it appeared in resp.Readings")
		}
	}

	// The recent reading (value 120) must appear
	found := false
	for _, r := range resp.Readings {
		if r.Value == 120 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("recent reading (120, 1h ago) should appear in 12h window response, but it did not")
	}
}

func TestDashboardAPI_AlarmHistory_InWindow(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := s.LogAlarmFired("noah", "Low", "brandon", now.Add(-1*time.Hour), 68); err != nil {
		t.Fatalf("LogAlarmFired: %v", err)
	}
	if err := s.LogAlarmFired("noah", "High", "brandon", now.Add(-25*time.Hour), 210); err != nil {
		t.Fatalf("LogAlarmFired (out of window): %v", err)
	}

	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard?window=24h")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.AlarmHistory) != 1 {
		t.Fatalf("expected 1 history entry in 24h window, got %d", len(resp.AlarmHistory))
	}
	if resp.AlarmHistory[0].AlarmName != "Low" {
		t.Errorf("AlarmName: got %q, want %q", resp.AlarmHistory[0].AlarmName, "Low")
	}
	if resp.AlarmHistory[0].BGValue != 68 {
		t.Errorf("BGValue: got %d, want 68", resp.AlarmHistory[0].BGValue)
	}
}

func TestDashboardAPI_AlarmHistory_EmptyForNoFirings(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 70, 180)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AlarmHistory == nil {
		t.Error("expected non-nil alarm_history slice (may be empty)")
	}
	if len(resp.AlarmHistory) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.AlarmHistory))
	}
}
