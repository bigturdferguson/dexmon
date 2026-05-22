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
