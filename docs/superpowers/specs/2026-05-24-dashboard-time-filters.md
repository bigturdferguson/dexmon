# Dashboard Time Filters Design

**Date:** 2026-05-24
**Status:** Approved

## Overview

Add a time-range filter to the dashboard. The user selects 12h, 24h, 7d, or 30d; the chart, stat widgets, and card title all reflect the selected window. Current and Previous BG readings are always the most recent and are unaffected. Alarms stay as-is. Selected range persists in `localStorage`.

---

## Backend

### API change

`GET /api/dashboard` gains an optional `?window=` query param:

| Value | Duration | Notes |
|-------|----------|-------|
| `12h` | 12 hours | |
| `24h` | 24 hours | default (missing or invalid → 24h) |
| `7d`  | 7 days (168h) | |
| `30d` | 30 days (720h) | |

The handler parses the param, maps it to a `time.Duration`, computes `since = now - duration`, and passes it to the existing `GetReadings` and `GetReadingStats` store calls. No store changes required.

### `DashboardResponse` change

Add one field:

```go
type DashboardResponse struct {
    Account  string        `json:"account"`
    AsOf     time.Time     `json:"as_of"`
    Window   string        `json:"window"`   // "12h" | "24h" | "7d" | "30d"
    Target   TargetJSON    `json:"target"`
    Current  *ReadingJSON  `json:"current"`
    Previous *ReadingJSON  `json:"previous"`
    Stats    StatsJSON     `json:"stats"`
    Readings []ReadingJSON `json:"readings"`
    Alarms   []AlarmJSON   `json:"alarms"`
}
```

`Window` echoes the validated window string back to the frontend.

### `serveAPI` logic

```go
func windowDuration(s string) (string, time.Duration) {
    switch s {
    case "12h": return "12h", 12 * time.Hour
    case "7d":  return "7d",  7 * 24 * time.Hour
    case "30d": return "30d", 30 * 24 * time.Hour
    default:    return "24h", 24 * time.Hour
    }
}
```

Called at the top of `serveAPI`:

```go
window, dur := windowDuration(r.URL.Query().Get("window"))
since := now.Add(-dur)
```

---

## Frontend

### Range pill buttons

Added to the chart card header, right-aligned inline with the card title:

```
BG — Last 24 Hours              [12h] [24h] [7d] [30d]
```

- Active button has a distinct style (filled/highlighted)
- Clicking a button: sets the active window, fetches `?window=X`, re-renders
- Selected window saved to `localStorage` key `dexmon-window`, restored on load (default `24h`)

### Stat widget subtitles

High, Low, and Avg stat cards currently show `24h` as a static subtitle. These update to reflect the active window: `12h`, `24h`, `7d`, or `30d`.

Current BG and Previous BG are unaffected — they always show the most recent readings.

### Chart card title

`"BG — Last 24 Hours"` becomes dynamic:

| Window | Title |
|--------|-------|
| 12h | BG — Last 12 Hours |
| 24h | BG — Last 24 Hours |
| 7d  | BG — Last 7 Days |
| 30d | BG — Last 30 Days |

### Adaptive chart labels

Label format and tick limit adapt to the window:

| Window | Label format | `maxTicksLimit` | Approximate spacing |
|--------|-------------|-----------------|---------------------|
| 12h | `14:00` | 6 | ~every 2h |
| 24h | `14:00` | 8 | ~every 3h (current) |
| 7d  | `Mon` / `Tue` etc. | 7 | ~one per day |
| 30d | `May 24` | 8 | ~every 4 days |

Label generation:

```js
function chartLabel(isoStr, window) {
    const dt = new Date(isoStr);
    if (window === '7d') {
        return dt.toLocaleDateString([], { weekday: 'short' });
    }
    if (window === '30d') {
        return dt.toLocaleDateString([], { month: 'short', day: 'numeric' });
    }
    return dt.getHours().toString().padStart(2, '0') + ':' +
           dt.getMinutes().toString().padStart(2, '0');
}
```

`maxRotation` remains `0` for 12h/24h. For 7d/30d, short day/date labels don't need rotation — `maxRotation: 0` stays for all windows.

### Point radius

Unchanged: `readings.length > 60 ? 2 : 3`. Already handles higher densities at 7d/30d.

---

## Tests

### `dashboard/handler_test.go`

- `GET /api/dashboard` with no `?window` → `resp.Window == "24h"`
- `GET /api/dashboard?window=7d` → `resp.Window == "7d"`
- `GET /api/dashboard?window=invalid` → `resp.Window == "24h"` (falls back to default)
- `GET /api/dashboard?window=12h` → `resp.Window == "12h"`

Existing tests: update `DashboardResponse` decode assertions to include `Window` where checked (no structural change needed — `Window` is additive).

---

## Out of Scope

- Alarm fire history / per-window trigger counts
- Custom date range picker
- Per-account default window in config
