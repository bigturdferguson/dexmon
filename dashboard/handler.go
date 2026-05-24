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
	"active":        4,
	"snoozed_until": 3,
	"fired":         2,
	"never_fired":   1,
}

// Handler serves the dashboard HTML and JSON API.
type Handler struct {
	store      *store.Store
	account    string
	alarms     []config.AlarmConfig
	recipients map[string]config.RecipientConfig // reserved for future recipient-name display
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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
