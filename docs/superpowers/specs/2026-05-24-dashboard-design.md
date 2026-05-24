# Dashboard Design

**Date:** 2026-05-24
**Status:** Approved

## Overview

A read-only web dashboard served at `https://dexmon.fly.dev/` showing real-time CGM data for a single account. The page auto-refreshes every 5 minutes, supports light and dark themes, and is responsive across desktop and mobile.

Authentication is out of scope for this iteration. The design is structured so that a single middleware wrapper on the HTTP mux will protect both routes when auth is added later.

---

## Architecture

**Approach:** JSON API + embedded vanilla JS (Option B).

- `GET /` — serves a self-contained `index.html` embedded in the binary via `go:embed`
- `GET /api/dashboard` — returns a JSON snapshot of the current account's data

Both routes are registered on the existing `http.ServeMux` in `callback/server.go`. No new server or port is introduced.

---

## Backend

### New store methods (`store/readings.go`)

```go
GetReadings(account string, since time.Time) ([]types.Reading, error)
// Returns all readings for the account since the given time, ordered by recorded_at ASC.

GetReadingStats(account string, since time.Time) (min, max, avg int, err error)
// Returns min, max, and integer average BG value over the window.
// Returns zeros and no error if no readings exist in the window.
```

### New package: `dashboard/`

`dashboard/handler.go` implements two handlers and is constructed with the store, account name, and slice of alarm configs:

```go
func New(st *store.Store, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig) http.Handler
```

**`GET /`** — serves `static/index.html` from the embedded filesystem. Sets `Content-Type: text/html; charset=utf-8`.

**`GET /api/dashboard`** — queries the store for the last 24 hours of data and returns:

```json
{
  "account": "noah",
  "as_of": "2026-05-24T14:32:00Z",
  "current":  { "value": 142, "trend": "flat",         "recorded_at": "2026-05-24T14:17:00Z" },
  "previous": { "value": 138, "trend": "forty_five_up", "recorded_at": "2026-05-24T14:12:00Z" },
  "stats": { "high": 187, "low": 72, "avg": 128 },
  "readings": [
    { "value": 72, "trend": "single_down", "recorded_at": "2026-05-23T14:32:00Z" }
  ],
  "alarms": [
    {
      "name": "Urgent Low",
      "priority": "emergency",
      "last_fired_at": "2026-05-24T12:15:00Z",
      "status": "fired"
    },
    {
      "name": "High",
      "priority": "high",
      "last_fired_at": "2026-05-24T08:00:00Z",
      "status": "snoozed_until",
      "snoozed_until": "2026-05-24T08:30:00Z"
    },
    {
      "name": "Low",
      "priority": "high",
      "last_fired_at": null,
      "status": "never_fired"
    }
  ]
}
```

**Alarm status logic** (computed server-side, evaluated in this order):
1. `last_fired_at` is null → `"never_fired"`
2. `receipt_id` is set and `receipt_expires_at` is in the future → `"active"` (emergency, awaiting acknowledgment)
3. `snoozed_until` is in the future → `"snoozed_until"` (include the timestamp)
4. Otherwise → `"fired"`

Note: there is no distinct `"acknowledged"` status. Once a Pushover emergency is acknowledged, the callback clears `receipt_id` — the state is indistinguishable from a non-emergency alarm that fired and is no longer active. Both fall through to `"fired"`.

**Multi-recipient alarms:** The alarm list shows one row per alarm name. For alarms with multiple recipients, the handler queries all (account, alarm_name, recipient) states and displays the most concerning status across recipients, using the priority order: `active` > `snoozed_until` > `fired` > `never_fired`.

`current` and `previous` are the two most recent entries from `readings` — no separate query. `as_of` is the server's current UTC time at the moment of the response.

Sets `Content-Type: application/json`. Returns `200` with an empty readings array and zero stats if no data exists — never a 4xx for missing data.

### Changes to existing files

**`callback/server.go`** — `New()` accepts additional parameters (`account string`, `alarms []config.AlarmConfig`, `recipients map[string]config.RecipientConfig`) and registers the dashboard handler:

```go
s.mux.Handle("GET /", dash)
s.mux.Handle("GET /api/dashboard", dash)
```

**`main.go`** — passes account name and alarm configs when constructing the server. Since the config only has one account, it extracts the single account name and its alarm list.

---

## Frontend (`dashboard/static/index.html`)

A single self-contained HTML file. No external dependencies — Chart.js is bundled as an inline minified `<script>` block.

### Layout

**Desktop** (≥640px): five stat widgets in a single row, full-width graph below, alarm list below that.

**Mobile** (<640px): current BG widget spans full width, remaining four stats in a 2×2 grid, graph full width, alarm list below.

### Widgets

| Widget | Content |
|--------|---------|
| Current BG | Large value + trend arrow + time since reading (e.g. "15m ago") |
| Previous | Value + time since reading |
| High | Max BG in window |
| Low | Min BG in window |
| Avg | Integer average BG in window |

**BG color coding** applied to the current BG value and the graph line:
- `< 70` — red
- `70–180` — green
- `> 180` — amber

### Graph

Chart.js line chart. X-axis: time over the 24-hour window. Y-axis: BG value. A shaded horizontal band marks the 70–180 target range. Points are colored by the same red/green/amber scheme. No chart legend needed.

### Alarm list

Columns: alarm name, priority badge, time since last fired (or "—"), status label.

Status display:
- `never_fired` → "—"
- `active` → "Active" (styled in red — emergency awaiting acknowledgment)
- `snoozed_until` → "Snoozed until HH:MM"
- `fired` → time since last fired (e.g. "2h ago")

### Auto-refresh

JS fetches `/api/dashboard` on page load, then again every 5 minutes via `setInterval`. On each successful fetch, all widgets, the chart, and the alarm list update in place without a page reload. The header shows "Last updated: Xm ago" derived from `as_of`, updating every minute via a separate `setInterval`.

If a fetch fails, the "Last updated" text changes to "Update failed" in amber — existing data stays visible.

### Theme toggle

A sun/moon button in the header toggles a `dark` class on `<body>`. The preference is stored in `localStorage` under the key `dexmon-theme` and restored on page load. All colors are defined as CSS custom properties — one `:root` block for light, one `body.dark` override for dark. No flash on load.

---

## Security

- Both endpoints are read-only; no user input is accepted in this iteration.
- All DB queries use parameterized statements.
- Chart.js is self-hosted (no CDN); no third-party scripts execute.
- `go:embed` reads files at compile time — no runtime path traversal.
- No cookies, sessions, or tokens.
- **Public exposure:** BG readings and alarm history are accessible without authentication. Auth middleware wrapping the mux is the intended future mitigation.

---

## Testing

- `store/store_test.go` — unit tests for `GetReadings` and `GetReadingStats` covering empty window, single reading, and multiple readings.
- `dashboard/handler_test.go` — tests for `GET /api/dashboard`: correct JSON shape, correct alarm status computation for each status variant, empty-data case returns 200 with zero stats.
- `GET /` is not unit-tested (it just serves a static file from embed).
- No frontend tests — the JS is render logic over a well-tested API.

---

## Out of Scope (this iteration)

- Authentication
- Time range filter (`?hours=N`) — API is designed to accept it later with minimal changes
- Multi-account support
- WebSocket / SSE live push — polling every 5 min matches the CGM update cadence
- Pushover notification actions from the dashboard
