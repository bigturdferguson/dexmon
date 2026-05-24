# Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only web dashboard at `/` showing real-time CGM data — stat widgets, a BG graph, and alarm status — with auto-refresh and light/dark theme support.

**Architecture:** A JSON API at `GET /api/dashboard` is consumed by a self-contained `index.html` served at `GET /`. Both routes are registered on the existing `http.ServeMux` in `callback/server.go`. Chart.js is vendored as a static file and served by the same handler. The HTML page fetches data on load and every 5 minutes without a full page reload.

**Tech Stack:** Go `net/http`, `go:embed`, Chart.js v4.4.4 (vendored), vanilla JS, CSS custom properties for theming.

---

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `store/readings.go` | Modify | Add `GetReadings`, `GetReadingStats` |
| `store/store_test.go` | Modify | Tests for new store methods |
| `dashboard/handler.go` | Create | HTTP handler, API logic, alarm status computation |
| `dashboard/handler_test.go` | Create | API response shape and alarm status tests |
| `dashboard/static/index.html` | Create | Self-contained dashboard (HTML + CSS + JS) |
| `dashboard/static/chart.min.js` | Create | Chart.js v4.4.4 minified (vendored) |
| `callback/server.go` | Modify | Updated `New()` signature, dashboard route registration |
| `callback/server_test.go` | Modify | Update `callback.New()` calls to new signature |
| `main.go` | Modify | Extract account + alarms, pass to server |

---

## Task 1: Store query methods

**Files:**
- Modify: `store/readings.go`
- Modify: `store/store_test.go`

- [ ] **Step 1.1: Write failing tests**

Add to the bottom of `store/store_test.go`:

```go
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
```

- [ ] **Step 1.2: Run tests to confirm they fail**

```bash
go test ./store/... -run "TestGetReadings|TestGetReadingStats" -v
```

Expected: FAIL with `s.GetReadings undefined` and `s.GetReadingStats undefined`.

- [ ] **Step 1.3: Implement the new store methods**

Add to the bottom of `store/readings.go`:

```go
func (s *Store) GetReadings(account string, since time.Time) ([]types.Reading, error) {
	rows, err := s.db.Query(
		`SELECT value, trend, recorded_at FROM readings
		 WHERE account = ? AND recorded_at >= ?
		 ORDER BY recorded_at ASC`,
		account, since.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var readings []types.Reading
	for rows.Next() {
		var r types.Reading
		var trend string
		var recordedAt time.Time
		if err := rows.Scan(&r.Value, &trend, &recordedAt); err != nil {
			return nil, err
		}
		r.Account = account
		r.Trend = types.Trend(trend)
		r.RecordedAt = recordedAt.UTC()
		readings = append(readings, r)
	}
	return readings, rows.Err()
}

func (s *Store) GetReadingStats(account string, since time.Time) (min, max, avg int, err error) {
	err = s.db.QueryRow(
		`SELECT COALESCE(MIN(value), 0), COALESCE(MAX(value), 0), COALESCE(CAST(AVG(value) AS INTEGER), 0)
		 FROM readings WHERE account = ? AND recorded_at >= ?`,
		account, since.UTC(),
	).Scan(&min, &max, &avg)
	return
}
```

- [ ] **Step 1.4: Run tests to confirm they pass**

```bash
go test ./store/... -v
```

Expected: all store tests PASS.

- [ ] **Step 1.5: Commit**

```bash
git add store/readings.go store/store_test.go
git commit -m "feat: add GetReadings and GetReadingStats store methods"
```

---

## Task 2: Dashboard handler

**Files:**
- Create: `dashboard/handler.go`
- Create: `dashboard/handler_test.go`
- Create: `dashboard/static/index.html` (placeholder — replaced in Task 3)
- Create: `dashboard/static/chart.min.js` (placeholder — replaced in Task 3)

- [ ] **Step 2.1: Create placeholder static files so embed compiles**

Create `dashboard/static/chart.min.js` with content:
```
// placeholder
```

Create `dashboard/static/index.html` with content:
```html
<!DOCTYPE html><html><body>dashboard</body></html>
```

- [ ] **Step 2.2: Write `dashboard/handler_test.go`**

```go
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
	h := dashboard.New(s, "noah", nil, nil)
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

	h := dashboard.New(s, "noah", nil, nil)
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

	h := dashboard.New(s, "noah", alarms, recipients)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

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
	h := dashboard.New(s, "noah", alarms, nil)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

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
	h := dashboard.New(s, "noah", alarms, nil)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

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
	h := dashboard.New(s, "noah", alarms, nil)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

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
	h := dashboard.New(s, "noah", alarms, nil)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Alarms[0].Status != "active" {
		t.Errorf("expected active (most concerning), got %q", resp.Alarms[0].Status)
	}
}

func TestDashboardIndex_ServesHTML(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil)
	w := get(t, h, "/")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content-type, got %q", ct)
	}
}
```

- [ ] **Step 2.3: Run tests to confirm they fail**

```bash
go test ./dashboard/... -v
```

Expected: FAIL with `cannot find package "dexmon/dashboard"`.

- [ ] **Step 2.4: Write `dashboard/handler.go`**

```go
package dashboard

import (
	"embed"
	"encoding/json"
	"net/http"
	"time"

	"dexmon/config"
	"dexmon/store"
	"dexmon/types"
)

//go:embed static
var staticFS embed.FS

// DashboardResponse is the JSON shape returned by GET /api/dashboard.
type DashboardResponse struct {
	Account  string        `json:"account"`
	AsOf     time.Time     `json:"as_of"`
	Current  *ReadingJSON  `json:"current"`
	Previous *ReadingJSON  `json:"previous"`
	Stats    StatsJSON     `json:"stats"`
	Readings []ReadingJSON `json:"readings"`
	Alarms   []AlarmJSON   `json:"alarms"`
}

type ReadingJSON struct {
	Value      int       `json:"value"`
	Trend      string    `json:"trend"`
	RecordedAt time.Time `json:"recorded_at"`
}

type StatsJSON struct {
	High int `json:"high"`
	Low  int `json:"low"`
	Avg  int `json:"avg"`
}

type AlarmJSON struct {
	Name         string     `json:"name"`
	Priority     string     `json:"priority"`
	LastFiredAt  *time.Time `json:"last_fired_at"`
	Status       string     `json:"status"`
	SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`
}

var statusRank = map[string]int{
	"active":       4,
	"snoozed_until": 3,
	"fired":        2,
	"never_fired":  1,
}

// Handler serves the dashboard HTML and JSON API.
type Handler struct {
	store      *store.Store
	account    string
	alarms     []config.AlarmConfig
	recipients map[string]config.RecipientConfig
}

// New constructs a Handler. Pass the single monitored account name and its
// alarm configs so the API can return per-alarm status.
func New(st *store.Store, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig) *Handler {
	return &Handler{store: st, account: account, alarms: alarms, recipients: recipients}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/dashboard":
		h.serveAPI(w, r)
	case "/chart.min.js":
		h.serveStatic(w, r, "static/chart.min.js", "application/javascript")
	default:
		h.serveStatic(w, r, "static/index.html", "text/html; charset=utf-8")
	}
}

func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request, path, contentType string) {
	data, err := staticFS.ReadFile(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

func (h *Handler) serveAPI(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	readings, err := h.store.GetReadings(h.account, since)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	minVal, maxVal, avgVal, err := h.store.GetReadingStats(h.account, since)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := DashboardResponse{
		Account:  h.account,
		AsOf:     now,
		Stats:    StatsJSON{High: maxVal, Low: minVal, Avg: avgVal},
		Readings: toReadingJSON(readings),
		Alarms:   h.buildAlarmList(now),
	}

	if n := len(readings); n > 0 {
		last := readings[n-1]
		resp.Current = &ReadingJSON{Value: last.Value, Trend: string(last.Trend), RecordedAt: last.RecordedAt}
	}
	if n := len(readings); n > 1 {
		prev := readings[n-2]
		resp.Previous = &ReadingJSON{Value: prev.Value, Trend: string(prev.Trend), RecordedAt: prev.RecordedAt}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) buildAlarmList(now time.Time) []AlarmJSON {
	type result struct {
		alarm        AlarmJSON
		rank         int
	}
	best := map[string]result{}
	order := []string{}

	for _, alarm := range h.alarms {
		if _, seen := best[alarm.Name]; !seen {
			order = append(order, alarm.Name)
			best[alarm.Name] = result{
				alarm: AlarmJSON{Name: alarm.Name, Priority: alarm.Priority, Status: "never_fired"},
				rank:  statusRank["never_fired"],
			}
		}
		for _, recipientName := range alarm.Recipients {
			state, err := h.store.GetAlarmState(h.account, alarm.Name, recipientName)
			if err != nil {
				continue
			}
			status, snoozedUntil := alarmStatus(now, state)
			rank := statusRank[status]
			if rank > best[alarm.Name].rank {
				best[alarm.Name] = result{
					alarm: AlarmJSON{
						Name:         alarm.Name,
						Priority:     alarm.Priority,
						LastFiredAt:  state.LastFiredAt,
						Status:       status,
						SnoozedUntil: snoozedUntil,
					},
					rank: rank,
				}
			}
		}
	}

	out := make([]AlarmJSON, 0, len(order))
	seen := map[string]bool{}
	for _, name := range order {
		if !seen[name] {
			out = append(out, best[name].alarm)
			seen[name] = true
		}
	}
	return out
}

func alarmStatus(now time.Time, state *types.AlarmState) (status string, snoozedUntil *time.Time) {
	if state.LastFiredAt == nil {
		return "never_fired", nil
	}
	if state.ReceiptID != nil && state.ReceiptExpiresAt != nil && state.ReceiptExpiresAt.After(now) {
		return "active", nil
	}
	if state.SnoozedUntil != nil && state.SnoozedUntil.After(now) {
		return "snoozed_until", state.SnoozedUntil
	}
	return "fired", nil
}

func toReadingJSON(readings []types.Reading) []ReadingJSON {
	out := make([]ReadingJSON, len(readings))
	for i, r := range readings {
		out[i] = ReadingJSON{Value: r.Value, Trend: string(r.Trend), RecordedAt: r.RecordedAt}
	}
	return out
}
```

- [ ] **Step 2.5: Run tests to confirm they pass**

```bash
go test ./dashboard/... -v
```

Expected: all dashboard tests PASS.

- [ ] **Step 2.6: Commit**

```bash
git add dashboard/
git commit -m "feat: add dashboard handler with JSON API and alarm status logic"
```

---

## Task 3: Static assets — Chart.js and full index.html

**Files:**
- Modify: `dashboard/static/chart.min.js` (replace placeholder)
- Modify: `dashboard/static/index.html` (replace placeholder)

- [ ] **Step 3.1: Download Chart.js v4.4.4 minified**

```bash
curl -sL https://cdn.jsdelivr.net/npm/chart.js@4.4.4/dist/chart.umd.min.js \
  -o dashboard/static/chart.min.js
```

Verify it downloaded (should be ~200KB):

```bash
wc -c dashboard/static/chart.min.js
```

Expected: output shows a byte count greater than 100000.

- [ ] **Step 3.2: Write `dashboard/static/index.html`**

Use the Write tool to create this file with the following content:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>dexmon</title>
  <script>
    (function() {
      if (localStorage.getItem('dexmon-theme') === 'dark') {
        document.documentElement.classList.add('dark');
      }
    })();
  </script>
  <script src="/chart.min.js"></script>
  <style>
    :root {
      --bg: #f8fafc;
      --surface: #ffffff;
      --border: #e2e8f0;
      --text: #1e293b;
      --text-muted: #64748b;
      --low: #dc2626;
      --high: #d97706;
      --normal: #16a34a;
      --low-line: rgba(220,38,38,0.45);
      --high-line: rgba(217,119,6,0.45);
      --grid: rgba(100,116,139,0.12);
      --shadow: 0 1px 3px rgba(0,0,0,0.07), 0 1px 2px rgba(0,0,0,0.05);
    }
    .dark {
      --bg: #0f172a;
      --surface: #1e293b;
      --border: #334155;
      --text: #f1f5f9;
      --text-muted: #94a3b8;
      --shadow: 0 1px 3px rgba(0,0,0,0.4);
    }
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: var(--bg);
      color: var(--text);
      min-height: 100vh;
      transition: background 0.2s, color 0.2s;
    }
    /* ── Header ── */
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0.875rem 1.5rem;
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      position: sticky;
      top: 0;
      z-index: 10;
    }
    header h1 { font-size: 1.125rem; font-weight: 700; letter-spacing: -0.01em; }
    .header-right { display: flex; align-items: center; gap: 0.875rem; }
    #last-updated { font-size: 0.8rem; color: var(--text-muted); }
    #theme-btn {
      background: none;
      border: 1px solid var(--border);
      border-radius: 0.375rem;
      padding: 0.25rem 0.625rem;
      cursor: pointer;
      font-size: 0.9rem;
      color: var(--text);
      line-height: 1.4;
    }
    /* ── Layout ── */
    main { max-width: 1200px; margin: 0 auto; padding: 1.25rem 1.5rem; }
    /* ── Stat grid ── */
    .stats-grid {
      display: grid;
      grid-template-columns: 2fr 1fr 1fr 1fr 1fr;
      gap: 0.875rem;
      margin-bottom: 1.25rem;
    }
    @media (max-width: 640px) {
      .stats-grid { grid-template-columns: 1fr 1fr; }
      .stat-current { grid-column: 1 / -1; }
    }
    .stat-card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 0.75rem;
      padding: 1rem 1.125rem;
      box-shadow: var(--shadow);
    }
    .stat-label {
      font-size: 0.6875rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-muted);
      margin-bottom: 0.5rem;
    }
    .stat-value {
      font-size: 2.25rem;
      font-weight: 700;
      line-height: 1;
      font-variant-numeric: tabular-nums;
    }
    .stat-card:not(.stat-current) .stat-value { font-size: 1.75rem; }
    .stat-sub {
      margin-top: 0.3rem;
      font-size: 0.8rem;
      color: var(--text-muted);
      display: flex;
      align-items: center;
      gap: 0.25rem;
    }
    .trend { font-size: 1.2rem; }
    /* BG color classes */
    .bg-low    { color: var(--low); }
    .bg-high   { color: var(--high); }
    .bg-normal { color: var(--normal); }
    /* ── Cards ── */
    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 0.75rem;
      padding: 1.125rem;
      box-shadow: var(--shadow);
      margin-bottom: 1.25rem;
    }
    .card-title {
      font-size: 0.6875rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-muted);
      margin-bottom: 1rem;
    }
    .chart-wrap { position: relative; height: 220px; }
    /* ── Alarm table ── */
    table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
    th {
      text-align: left;
      padding: 0.4rem 0.75rem;
      font-size: 0.6875rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-muted);
      border-bottom: 1px solid var(--border);
    }
    td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border); }
    tr:last-child td { border-bottom: none; }
    .badge {
      display: inline-block;
      padding: 0.1rem 0.45rem;
      border-radius: 9999px;
      font-size: 0.6875rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .badge-emergency { background: rgba(220,38,38,0.1);  color: var(--low); }
    .badge-high      { background: rgba(217,119,6,0.1);  color: var(--high); }
    .badge-normal    { background: rgba(22,163,74,0.1);  color: var(--normal); }
    .status-active { color: var(--low); font-weight: 600; }
    .text-muted { color: var(--text-muted); }
    .update-error { color: var(--high); }
  </style>
</head>
<body>
  <header>
    <h1>dexmon</h1>
    <div class="header-right">
      <span id="last-updated">Loading…</span>
      <button id="theme-btn" onclick="toggleTheme()">☀︎</button>
    </div>
  </header>
  <main>
    <div class="stats-grid">
      <div class="stat-card stat-current">
        <div class="stat-label">Current BG</div>
        <div class="stat-value" id="val-current">—</div>
        <div class="stat-sub">
          <span class="trend" id="val-trend"></span>
          <span id="val-current-age"></span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Previous</div>
        <div class="stat-value" id="val-prev">—</div>
        <div class="stat-sub" id="val-prev-age"></div>
      </div>
      <div class="stat-card">
        <div class="stat-label">High</div>
        <div class="stat-value" id="val-high">—</div>
        <div class="stat-sub">24h</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Low</div>
        <div class="stat-value" id="val-low">—</div>
        <div class="stat-sub">24h</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Avg</div>
        <div class="stat-value" id="val-avg">—</div>
        <div class="stat-sub">24h</div>
      </div>
    </div>

    <div class="card">
      <div class="card-title">BG — Last 24 Hours</div>
      <div class="chart-wrap"><canvas id="bg-chart"></canvas></div>
    </div>

    <div class="card">
      <div class="card-title">Alarms</div>
      <table>
        <thead>
          <tr><th>Alarm</th><th>Priority</th><th>Last Fired</th><th>Status</th></tr>
        </thead>
        <tbody id="alarm-rows">
          <tr><td colspan="4" class="text-muted">Loading…</td></tr>
        </tbody>
      </table>
    </div>
  </main>

  <script>
    const REFRESH_MS = 5 * 60 * 1000;
    const TREND_ARROW = {
      double_up: '↑↑', single_up: '↑', forty_five_up: '↗',
      flat: '→',
      forty_five_down: '↘', single_down: '↓', double_down: '↓↓',
    };

    let chart = null;
    let lastData = null;
    let lastAsOf = null;

    // ── theme ──────────────────────────────────────────────────────────────
    function applyTheme() {
      const dark = document.documentElement.classList.contains('dark');
      document.getElementById('theme-btn').textContent = dark ? '☾' : '☀︎';
    }
    function toggleTheme() {
      document.documentElement.classList.toggle('dark');
      const dark = document.documentElement.classList.contains('dark');
      localStorage.setItem('dexmon-theme', dark ? 'dark' : 'light');
      applyTheme();
      if (chart) { chart.destroy(); chart = null; }
      if (lastData) renderChart(lastData.readings || []);
    }
    applyTheme();

    // ── helpers ────────────────────────────────────────────────────────────
    function cssVar(name) {
      return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    }
    function bgClass(v) {
      if (v < 70) return 'bg-low';
      if (v > 180) return 'bg-high';
      return 'bg-normal';
    }
    function bgColor(v) {
      if (v < 70) return cssVar('--low');
      if (v > 180) return cssVar('--high');
      return cssVar('--normal');
    }
    function timeAgo(iso) {
      if (!iso) return '';
      const s = Math.floor((Date.now() - new Date(iso)) / 1000);
      if (s < 60)   return s + 's ago';
      if (s < 3600) return Math.floor(s / 60) + 'm ago';
      if (s < 86400) return Math.floor(s / 3600) + 'h ago';
      return Math.floor(s / 86400) + 'd ago';
    }
    function fmtTime(iso) {
      return new Date(iso).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
    }
    function esc(s) {
      return String(s)
        .replace(/&/g,'&amp;').replace(/</g,'&lt;')
        .replace(/>/g,'&gt;').replace(/"/g,'&quot;');
    }

    // ── render ─────────────────────────────────────────────────────────────
    function renderWidgets(d) {
      const cur = d.current, prev = d.previous;
      const curEl = document.getElementById('val-current');
      curEl.textContent = cur ? cur.value : '—';
      curEl.className = 'stat-value' + (cur ? ' ' + bgClass(cur.value) : '');
      document.getElementById('val-trend').textContent = cur ? (TREND_ARROW[cur.trend] || '') : '';
      document.getElementById('val-current-age').textContent = cur ? timeAgo(cur.recorded_at) : '';

      const prevEl = document.getElementById('val-prev');
      prevEl.textContent = prev ? prev.value : '—';
      prevEl.className = 'stat-value' + (prev ? ' ' + bgClass(prev.value) : '');
      document.getElementById('val-prev-age').textContent = prev ? timeAgo(prev.recorded_at) : '';

      document.getElementById('val-high').textContent = d.stats.high || '—';
      document.getElementById('val-low').textContent  = d.stats.low  || '—';
      document.getElementById('val-avg').textContent  = d.stats.avg  || '—';
    }

    function renderChart(readings) {
      if (!readings.length) return;
      const labels = readings.map(r => {
        const dt = new Date(r.recorded_at);
        return dt.getHours().toString().padStart(2,'0') + ':' +
               dt.getMinutes().toString().padStart(2,'0');
      });
      const values = readings.map(r => r.value);
      const ptColors = readings.map(r => bgColor(r.value));
      const ref = new Array(readings.length);

      const lowLine  = Array(readings.length).fill(70);
      const highLine = Array(readings.length).fill(180);

      if (chart) {
        chart.data.labels = labels;
        chart.data.datasets[0].data = values;
        chart.data.datasets[0].pointBackgroundColor = ptColors;
        chart.data.datasets[1].data = lowLine;
        chart.data.datasets[2].data = highLine;
        chart.update('none');
        return;
      }

      const ctx = document.getElementById('bg-chart').getContext('2d');
      chart = new Chart(ctx, {
        type: 'line',
        data: {
          labels,
          datasets: [
            {
              data: values,
              borderColor: '#6366f1',
              borderWidth: 2,
              pointBackgroundColor: ptColors,
              pointRadius: readings.length > 60 ? 2 : 3,
              tension: 0.3,
              fill: false,
            },
            {
              data: lowLine,
              borderColor: cssVar('--low-line'),
              borderWidth: 1,
              borderDash: [5, 4],
              pointRadius: 0,
              fill: false,
            },
            {
              data: highLine,
              borderColor: cssVar('--high-line'),
              borderWidth: 1,
              borderDash: [5, 4],
              pointRadius: 0,
              fill: false,
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { display: false },
            tooltip: {
              filter: item => item.datasetIndex === 0,
              callbacks: { label: ctx => 'BG: ' + ctx.parsed.y },
            },
          },
          scales: {
            x: {
              grid: { color: cssVar('--grid') },
              ticks: { color: cssVar('--text-muted'), maxTicksLimit: 8, maxRotation: 0 },
            },
            y: {
              suggestedMin: 40,
              grid: { color: cssVar('--grid') },
              ticks: { color: cssVar('--text-muted') },
            },
          },
        },
      });
    }

    function renderAlarms(alarms) {
      const tbody = document.getElementById('alarm-rows');
      if (!alarms || !alarms.length) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-muted">No alarms configured.</td></tr>';
        return;
      }
      tbody.innerHTML = alarms.map(a => {
        const badge = `<span class="badge badge-${esc(a.priority)}">${esc(a.priority)}</span>`;
        const lastFiredCell = a.last_fired_at ? timeAgo(a.last_fired_at) : '<span class="text-muted">—</span>';
        let statusCell;
        switch (a.status) {
          case 'never_fired':
            statusCell = '<span class="text-muted">—</span>'; break;
          case 'active':
            statusCell = '<span class="status-active">Active</span>'; break;
          case 'snoozed_until':
            statusCell = 'Snoozed until ' + fmtTime(a.snoozed_until); break;
          default: // 'fired'
            statusCell = a.last_fired_at ? timeAgo(a.last_fired_at) : '<span class="text-muted">—</span>';
        }
        return `<tr>
          <td>${esc(a.name)}</td>
          <td>${badge}</td>
          <td>${lastFiredCell}</td>
          <td>${statusCell}</td>
        </tr>`;
      }).join('');
    }

    function updateLastUpdated() {
      if (!lastAsOf) return;
      document.getElementById('last-updated').textContent = 'Last updated: ' + timeAgo(lastAsOf);
    }

    // ── fetch / refresh ────────────────────────────────────────────────────
    async function fetchAndRender() {
      try {
        const resp = await fetch('/api/dashboard');
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        const d = await resp.json();
        lastData = d;
        lastAsOf = d.as_of;
        renderWidgets(d);
        renderChart(d.readings || []);
        renderAlarms(d.alarms || []);
        updateLastUpdated();
        const el = document.getElementById('last-updated');
        el.className = '';
      } catch (e) {
        const el = document.getElementById('last-updated');
        el.textContent = 'Update failed';
        el.className = 'update-error';
      }
    }

    fetchAndRender();
    setInterval(fetchAndRender, REFRESH_MS);
    setInterval(updateLastUpdated, 60 * 1000);
  </script>
</body>
</html>
```

- [ ] **Step 3.3: Run tests to confirm nothing broke**

```bash
go test ./... -v
```

Expected: all tests PASS (replacing the static files doesn't affect Go tests).

- [ ] **Step 3.4: Commit**

```bash
git add dashboard/static/
git commit -m "feat: add dashboard static assets (Chart.js + index.html)"
```

---

## Task 4: Wire up server and main

**Files:**
- Modify: `callback/server.go`
- Modify: `callback/server_test.go`
- Modify: `main.go`

- [ ] **Step 4.1: Update `callback/server.go`**

Replace the entire file with:

```go
package callback

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"dexmon/config"
	"dexmon/dashboard"
	"dexmon/store"
)

type Server struct {
	store *store.Store
	port  int
	mux   *http.ServeMux
}

func New(st *store.Store, port int, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig) *Server {
	s := &Server{store: st, port: port, mux: http.NewServeMux()}
	dash := dashboard.New(st, account, alarms, recipients)
	s.mux.Handle("GET /", dash)
	s.mux.Handle("GET /api/dashboard", dash)
	s.mux.HandleFunc("POST /pushover/callback", s.handleCallback)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("callback server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

type callbackPayload struct {
	Receipt        string `json:"receipt"`
	AcknowledgedAt int64  `json:"acknowledged_at"`
	Snooze         int    `json:"snooze"`
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
	} else {
		state.SnoozedUntil = nil
	}

	if err := s.store.UpsertAlarmState(*state); err != nil {
		log.Printf("callback: update state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if payload.Snooze > 0 {
		snooze := time.Duration(payload.Snooze) * time.Second
		log.Printf("callback: %s/%q/%s acknowledged, snoozed %s", state.Account, state.AlarmName, state.Recipient, snooze)
	} else {
		log.Printf("callback: %s/%q/%s acknowledged", state.Account, state.AlarmName, state.Recipient)
	}
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4.2: Update all `callback.New()` calls in `callback/server_test.go`**

In `callback/server_test.go`, every occurrence of `callback.New(st, 0)` becomes `callback.New(st, 0, "", nil, nil)`. There are five occurrences — in `TestCallback_ClearsReceiptOnAck`, `TestCallback_SetsSnoozedUntilWhenSnoozeProvided`, `TestCallback_ClearsPreexistingSnoozeOnAckWithoutSnooze`, `TestCallback_IgnoresUnknownReceipt`, and `TestCallback_LogsAcknowledgment`.

- [ ] **Step 4.3: Update `main.go`**

Replace the `callback.New(...)` call at the bottom of `main()` and add account extraction just before it. The final `main.go` should look like:

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

	logStartup(cfg)

	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	disp := dispatcher.New(appToken, st, cfg.Server.CallbackURL)

	// Extract the single monitored account for the dashboard.
	var accountName string
	var accountAlarms []config.AlarmConfig
	for name, acct := range cfg.Accounts {
		accountName = name
		accountAlarms = acct.Alarms
		break
	}

	for name, acctCfg := range cfg.Accounts {
		client := dexcom.New(acctCfg.DexcomUsername, acctCfg.DexcomPassword)
		p := poller.New(name, acctCfg, client, st, disp, cfg.Recipients, cfg.Health)
		go p.Run()
	}

	srv := callback.New(st, cfg.Server.CallbackPort, accountName, accountAlarms, cfg.Recipients)
	if err := srv.Start(); err != nil {
		st.Close()
		log.Fatal(err)
	}
}

func logStartup(cfg *config.Config) {
	if cfg.Server.CallbackURL != "" {
		log.Printf("config: callback URL: %s", cfg.Server.CallbackURL)
	} else {
		log.Printf("config: callback URL: (not set — emergency callbacks disabled)")
	}
	for name, acct := range cfg.Accounts {
		log.Printf("config: account %q polling every %s, %d alarms", name, acct.PollInterval, len(acct.Alarms))
	}
	for name := range cfg.Recipients {
		log.Printf("config: recipient %q configured", name)
	}
	if cfg.Health.Watchdog.PingURL != "" {
		log.Printf("config: watchdog ping URL: %s", cfg.Health.Watchdog.PingURL)
	} else {
		log.Printf("config: watchdog ping: (not set)")
	}
}
```

- [ ] **Step 4.4: Run all tests**

```bash
go test ./...
```

Expected: all tests PASS with no compilation errors.

- [ ] **Step 4.5: Confirm the binary builds**

```bash
go build ./...
```

Expected: exits 0 with no output.

- [ ] **Step 4.6: Commit**

```bash
git add callback/server.go callback/server_test.go main.go
git commit -m "feat: wire dashboard into server and main"
```

---

## Task 5: Final verification and push

- [ ] **Step 5.1: Run the full test suite one more time**

```bash
go test ./... -count=1
```

Expected: all packages PASS, no cached results.

- [ ] **Step 5.2: Push**

```bash
git push
```
