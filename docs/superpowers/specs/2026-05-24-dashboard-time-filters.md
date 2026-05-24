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

### Adaptive chart labels — ruler-style ticks

Labels only appear at clean time boundaries. Sub-boundary positions show a short tick mark with no label (ruler effect). `maxTicksLimit` is removed; `autoSkip: false` is used instead so label generation is fully explicit.

**Major ticks (labeled):** only at clean boundaries per window:

| Window | Major tick condition | Label format |
|--------|---------------------|--------------|
| 12h / 24h | `minutes === 0` (exact hour) | `14:00` |
| 7d | `hours === 0 && minutes === 0` (midnight) | `Mon`, `Tue` etc. |
| 30d | `hours === 0 && minutes === 0` (midnight) | `May 24` |

**Minor ticks (short mark, no label):** drawn by a custom `rulerPlugin` after Chart.js renders:

| Window | Minor tick condition |
|--------|---------------------|
| 12h / 24h | `minutes === 30` |
| 7d | `hours === 12 && minutes === 0` |
| 30d | every other midnight (every 48h) |

All other readings: no tick mark, no label.

**Label generation** (passed to Chart.js as the `labels` array):

```js
function chartLabel(isoStr, window) {
    const dt = new Date(isoStr);
    if (window === '12h' || window === '24h') {
        return dt.getMinutes() === 0
            ? dt.getHours().toString().padStart(2, '0') + ':00'
            : '';
    }
    if (window === '7d') {
        return (dt.getHours() === 0 && dt.getMinutes() === 0)
            ? dt.toLocaleDateString([], { weekday: 'short' })
            : '';
    }
    // 30d
    return (dt.getHours() === 0 && dt.getMinutes() === 0)
        ? dt.toLocaleDateString([], { month: 'short', day: 'numeric' })
        : '';
}
```

**`rulerPlugin`** — a Chart.js plugin registered inline. After each draw, it iterates the readings array and draws a short vertical line (4px) on the x-axis at minor tick positions:

```js
const rulerPlugin = {
    id: 'ruler',
    afterDraw(chart) {
        const xScale = chart.scales.x;
        const ctx = chart.ctx;
        const window = chart._rulerWindow; // set before chart creation
        const readings = chart._rulerReadings;
        ctx.save();
        ctx.strokeStyle = getComputedStyle(document.documentElement)
            .getPropertyValue('--text-muted').trim();
        ctx.lineWidth = 1;
        readings.forEach((r, i) => {
            if (!isMinorTick(r.recorded_at, window)) return;
            const x = xScale.getPixelForValue(i);
            const y = xScale.bottom;
            ctx.beginPath();
            ctx.moveTo(x, y);
            ctx.lineTo(x, y + 4);
            ctx.stroke();
        });
        ctx.restore();
    }
};
```

`isMinorTick(isoStr, window)` returns `true` for the minor-tick conditions in the table above.

`maxRotation: 0` remains for all windows — clean boundary labels are short and never need rotation.

Chart.js x-axis options change from:
```js
ticks: { color: cssVar('--text-muted'), maxTicksLimit: 8, maxRotation: 0 }
```
to:
```js
ticks: { color: cssVar('--text-muted'), autoSkip: false, maxRotation: 0 }
```

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
