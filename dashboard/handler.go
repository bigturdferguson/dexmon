package dashboard

import (
	"embed"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"time"

	"dexmon/config"
	"dexmon/store"
	"dexmon/types"
)

//go:embed static
var staticFS embed.FS

type TargetJSON struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

// DashboardResponse is the JSON shape returned by GET /api/dashboard.
type DashboardResponse struct {
	Account      string             `json:"account"`
	AsOf         time.Time          `json:"as_of"`
	Window       string             `json:"window"`
	Target       TargetJSON         `json:"target"`
	Current      *ReadingJSON       `json:"current"`
	Previous     *ReadingJSON       `json:"previous"`
	Stats        StatsJSON          `json:"stats"`
	Readings     []ReadingJSON      `json:"readings"`
	Alarms       []AlarmJSON        `json:"alarms"`
	AlarmHistory []AlarmHistoryJSON `json:"alarm_history"`
	Health       HealthJSON         `json:"health"`
}

type ReadingJSON struct {
	Value      int       `json:"value"`
	Trend      string    `json:"trend"`
	RecordedAt time.Time `json:"recorded_at"`
}

type StatsJSON struct {
	High           int     `json:"high"`
	Low            int     `json:"low"`
	Avg            int     `json:"avg"`
	StdDev         int     `json:"std_dev"`
	CV             float64 `json:"cv"`
	TimeInRange    float64 `json:"time_in_range"`
	TimeBelowRange float64 `json:"time_below_range"`
	TimeAboveRange float64 `json:"time_above_range"`
	Q1             int     `json:"q1"`
	Median         int     `json:"median"`
	Q3             int     `json:"q3"`
}

type WatchdogHealthJSON struct {
	Configured bool    `json:"configured"`
	LastPingAt *string `json:"last_ping_at,omitempty"`
}

type HealthJSON struct {
	Watchdog WatchdogHealthJSON `json:"watchdog"`
}

func computeStats(readings []types.Reading, targetLow, targetHigh int) StatsJSON {
	n := len(readings)
	if n == 0 {
		return StatsJSON{}
	}

	var sum, sumSq float64
	minVal, maxVal := readings[0].Value, readings[0].Value
	inRange, belowRange, aboveRange := 0, 0, 0
	vals := make([]int, n)

	for i, r := range readings {
		v := r.Value
		vals[i] = v
		fv := float64(v)
		sum += fv
		sumSq += fv * fv
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		if v >= targetLow && v <= targetHigh {
			inRange++
		} else if v < targetLow {
			belowRange++
		} else {
			aboveRange++
		}
	}

	fn := float64(n)
	mean := sum / fn
	variance := sumSq/fn - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)

	var cv float64
	if mean > 0 {
		cv = math.Round(stddev/mean*100*10) / 10
	}
	tir := math.Round(float64(inRange)/fn*100*10) / 10
	tbr := math.Round(float64(belowRange)/fn*100*10) / 10
	tar := math.Round(float64(aboveRange)/fn*100*10) / 10

	sort.Ints(vals)
	q1 := vals[int(float64(n-1)*0.25)]
	median := vals[int(float64(n-1)*0.50)]
	q3 := vals[int(float64(n-1)*0.75)]

	return StatsJSON{
		High:           maxVal,
		Low:            minVal,
		Avg:            int(math.Round(mean)),
		StdDev:         int(math.Round(stddev)),
		CV:             cv,
		TimeInRange:    tir,
		TimeBelowRange: tbr,
		TimeAboveRange: tar,
		Q1:             q1,
		Median:         median,
		Q3:             q3,
	}
}

type AlarmJSON struct {
	Name         string     `json:"name"`
	Priority     string     `json:"priority"`
	LastFiredAt  *time.Time `json:"last_fired_at"`
	Status       string     `json:"status"`
	SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`
}

type AlarmHistoryJSON struct {
	AlarmName string    `json:"alarm_name"`
	FiredAt   time.Time `json:"fired_at"`
	BGValue   int       `json:"bg_value"`
}

var statusRank = map[string]int{
	"active":        4,
	"snoozed_until": 3,
	"fired":         2,
	"never_fired":   1,
}

// Handler serves the dashboard HTML and JSON API.
type Handler struct {
	store        *store.Store
	account      string
	alarms       []config.AlarmConfig
	recipients   map[string]config.RecipientConfig
	targetLow    int
	targetHigh   int
	watchdogURL  string
}

// New constructs a Handler. Pass the single monitored account name and its
// alarm configs so the API can return per-alarm status.
func New(st *store.Store, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int, watchdogURL string) *Handler {
	return &Handler{store: st, account: account, alarms: alarms, recipients: recipients, targetLow: targetLow, targetHigh: targetHigh, watchdogURL: watchdogURL}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/dashboard":
		h.serveAPI(w, r)
	case "/chart.min.js":
		h.serveStatic(w, r, "static/chart.min.js", "application/javascript")
	case "/favicon.ico":
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="14" fill="#6366f1"/></svg>`))
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
	if path == "static/chart.min.js" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Write(data)
}

func windowDuration(s string) (string, time.Duration) {
	switch s {
	case "1h":
		return "1h", 1 * time.Hour
	case "3h":
		return "3h", 3 * time.Hour
	case "6h":
		return "6h", 6 * time.Hour
	case "12h":
		return "12h", 12 * time.Hour
	case "7d":
		return "7d", 7 * 24 * time.Hour
	case "30d":
		return "30d", 30 * 24 * time.Hour
	case "90d":
		return "90d", 90 * 24 * time.Hour
	default:
		return "24h", 24 * time.Hour
	}
}

func (h *Handler) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := time.Now().UTC()
	window, dur := windowDuration(r.URL.Query().Get("window"))
	since := now.Add(-dur)

	readings, err := h.store.GetReadings(h.account, since)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	history, err := h.store.GetAlarmHistory(h.account, since)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := DashboardResponse{
		Account:      h.account,
		AsOf:         now,
		Window:       window,
		Target:       TargetJSON{Low: h.targetLow, High: h.targetHigh},
		Stats:        computeStats(readings, h.targetLow, h.targetHigh),
		Readings:     toReadingJSON(readings),
		Alarms:       h.buildAlarmList(now),
		AlarmHistory: toAlarmHistoryJSON(history),
	}

	if n := len(readings); n > 0 {
		last := readings[n-1]
		resp.Current = &ReadingJSON{Value: last.Value, Trend: string(last.Trend), RecordedAt: last.RecordedAt}
	}
	if n := len(readings); n > 1 {
		prev := readings[n-2]
		resp.Previous = &ReadingJSON{Value: prev.Value, Trend: string(prev.Trend), RecordedAt: prev.RecordedAt}
	}

	health := HealthJSON{
		Watchdog: WatchdogHealthJSON{Configured: h.watchdogURL != ""},
	}
	if h.watchdogURL != "" {
		if v, ok, err := h.store.GetMeta("last_watchdog_ping"); ok && err == nil {
			health.Watchdog.LastPingAt = &v
		}
	}
	resp.Health = health

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) buildAlarmList(now time.Time) []AlarmJSON {
	type result struct {
		alarm AlarmJSON
		rank  int
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
	for _, name := range order {
		out = append(out, best[name].alarm)
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

func toAlarmHistoryJSON(entries []types.AlarmHistoryEntry) []AlarmHistoryJSON {
	out := make([]AlarmHistoryJSON, len(entries))
	for i, e := range entries {
		out[i] = AlarmHistoryJSON{
			AlarmName: e.AlarmName,
			FiredAt:   e.FiredAt,
			BGValue:   e.BGValue,
		}
	}
	return out
}
