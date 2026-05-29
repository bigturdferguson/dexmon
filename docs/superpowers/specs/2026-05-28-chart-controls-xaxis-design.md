# Chart Controls & X-Axis Labels — Design Spec

**Date:** 2026-05-28
**File:** `dashboard/static/index.html`, `dashboard/handler.go`

---

## Summary

Three improvements to the BG line chart:

1. **Time-range control** — replace five pill buttons with a custom styled dropdown grouped into Short (1h · 3h · 6h · 12h) and Long (24h · 7d · 30d · 90d) ranges.
2. **X-axis labels** — every window shows the start and end time, intermediate ticks scaled to the window, and a suppression zone that prevents ticks from crowding either endpoint.
3. **Render tweaks** — dots only on 1h and 3h; LTTB sampling from 6h up; moving-average trend line on 7d / 30d / 90d.

---

## 1. Time-Range Dropdown

### HTML

Replace the `.window-pills` div and its five `<button class="pill">` children with:

```html
<div class="wsel-wrap" id="window-select">
  <button class="wsel-btn" id="wsel-btn" onclick="toggleWindowSelect()">
    <span id="wsel-label">24h</span>
    <span class="wsel-chevron">▼</span>
  </button>
  <div class="wsel-menu" id="wsel-menu">
    <div class="wsel-group">Short</div>
    <div class="wsel-item" data-window="1h"  onclick="setWindow('1h')">1h</div>
    <div class="wsel-item" data-window="3h"  onclick="setWindow('3h')">3h</div>
    <div class="wsel-item" data-window="6h"  onclick="setWindow('6h')">6h</div>
    <div class="wsel-item" data-window="12h" onclick="setWindow('12h')">12h</div>
    <div class="wsel-divider"></div>
    <div class="wsel-group">Long</div>
    <div class="wsel-item" data-window="24h" onclick="setWindow('24h')">24h</div>
    <div class="wsel-item" data-window="7d"  onclick="setWindow('7d')">7d</div>
    <div class="wsel-item" data-window="30d" onclick="setWindow('30d')">30d</div>
    <div class="wsel-item" data-window="90d" onclick="setWindow('90d')">90d</div>
  </div>
</div>
```

### CSS

Remove `.window-pills`, `.pill`, `.pill.active`. Add:

```css
.wsel-wrap { position: relative; display: inline-block; }
.wsel-btn {
  display: flex; align-items: center; gap: 0.3rem;
  background: var(--surface); border: 1px solid var(--border);
  border-radius: 0.375rem; padding: 0.2rem 0.5rem 0.2rem 0.6rem;
  font-size: 0.75rem; font-weight: 600; color: var(--text);
  cursor: pointer; white-space: nowrap; line-height: 1.4;
}
.wsel-btn:hover { border-color: var(--text-muted); }
.wsel-chevron { font-size: 0.5rem; color: var(--text-muted); margin-top: 1px; }
.wsel-menu {
  display: none; position: absolute; top: calc(100% + 4px); right: 0;
  background: var(--surface); border: 1px solid var(--border);
  border-radius: 0.5rem; box-shadow: 0 4px 16px rgba(0,0,0,0.11);
  min-width: 115px; z-index: 50; overflow: hidden;
}
.wsel-wrap.open .wsel-menu { display: block; }
.wsel-group {
  padding: 0.3rem 0.7rem 0.05rem;
  font-size: 0.59rem; font-weight: 700; text-transform: uppercase;
  letter-spacing: 0.08em; color: var(--text-muted);
}
.wsel-item {
  padding: 0.36rem 0.7rem; font-size: 0.775rem; font-weight: 500;
  color: var(--text); cursor: pointer;
  display: flex; align-items: center; justify-content: space-between;
}
.wsel-item:hover { background: var(--bg); }
.wsel-item.active { font-weight: 700; color: #6366f1; }
.wsel-item.active::after { content: '✓'; font-size: 0.68rem; }
.wsel-divider { height: 1px; background: var(--border); margin: 0.2rem 0; }
```

### JS: `toggleWindowSelect` and `updateWindowLabels`

`toggleWindowSelect()` toggles the `.open` class on `#window-select`. A document click listener removes `.open` when clicking outside the element.

`updateWindowLabels(w)` (existing function) must be updated:
- Set `#wsel-label` text to `w`
- Toggle `.active` class on `.wsel-item` elements matching `data-window === w`
- Remove the old `document.querySelectorAll('.pill')` call

`setWindow(w)` closes the menu before calling `fetchAndRender()` (same as before).

---

## 2. Backend: New Windows

`dashboard/handler.go` — `windowDuration` function adds three cases before the default:

```go
case "1h":
    return "1h", 1 * time.Hour
case "3h":
    return "3h", 3 * time.Hour
case "6h":
    return "6h", 6 * time.Hour
```

No other backend changes.

---

## 3. X-Axis Labels

### Strategy

- **Short windows** (1h · 3h · 6h · 12h · 24h): iterate data points; label start/end and any point whose timestamp falls on a tick boundary. Suppress boundary ticks within a minimum gap of the endpoints.
- **Long windows** (7d · 30d · 90d): generate explicit tick timestamps first, then snap each to the nearest data point index (LTTB may not preserve exact midnight/weekly/monthly boundaries). Suppress ticks within minimum gap of endpoints.

### Tick intervals

| Window | Tick every | Start/end format | Intermediate format |
|--------|-----------|-----------------|---------------------|
| 1h | 15 min | `HH:MM` | `HH:MM` |
| 3h | 30 min | `HH:MM` | `HH:MM` |
| 6h | 60 min | `HH:MM` | `HH:MM` |
| 12h | 2 h | `HH:MM` | `HH:MM` |
| 24h | 4 h | `HH:MM` | `HH:MM` |
| 7d | 1 day (midnight) | `MMM D` | `MMM D` |
| 30d | 7 days (from first midnight) | `MMM D` | `MMM D` |
| 90d | 1st of month | `MMM` | `MMM` |

### Suppression zones (minimum gap from endpoint)

| Window | Gap |
|--------|-----|
| 1h | 7.5 min |
| 3h | 15 min |
| 6h | 30 min |
| 12h | 60 min |
| 24h | 2 h |
| 7d | 24 h (prevents midnight = endpoint collision) |
| 30d | 4 days |
| 90d | 16 days |

### Long window tick generation

```
7d:  midnight of each calendar day within window (start+1 day, step 1 day)
30d: first midnight after window start, then every 7 days
90d: 1st of each month after window start
```

For each generated tick: find nearest index in display readings (binary/linear scan — readings are sorted). Skip if index already labeled. Assign `MMM D` (or `MMM` for 90d) label using the tick timestamp (not the data point's timestamp).

### Minor tick marks (ruler plugin)

| Window | Minor tick at |
|--------|--------------|
| 1h | every 5 min that is not a 15-min boundary |
| 3h | every 15 and 45 min |
| 6h / 12h / 24h | every :30 min |
| 7d / 30d / 90d | no minor ticks |

---

## 4. Render Tweaks

### Dots

Show data point dots only when `window ∈ {1h, 3h}`. All other windows: `pointRadius: 0`.

Point radius: `rds.length > 60 ? 2 : 3` (same logic as existing 12h/24h dot sizing, now applied only to 1h/3h).

### LTTB sampling

```
prepareChartData(readings, w):
  if w ∈ {1h, 3h, 6h, 12h, 24h} or readings.length ≤ 200:
    return readings   (no downsampling)
  return lttb(readings, 200)
```

(Previously only `12h` was excluded from downsampling; now all short windows are.)

### Trend line (moving average)

For `w ∈ {7d, 30d, 90d}`, add a 4th Chart.js dataset:

```js
{
  data: movingAverage(displayReadings, 0.12),  // 12% of dataset as smoothing window
  borderColor: 'rgba(99,102,241,0.5)',
  borderWidth: 2.5,
  pointRadius: 0,
  tension: 0.4,
  fill: false,
}
```

`movingAverage(data, pct)`: for each point `i`, average over a centered window of `max(3, round(data.length × pct))` values.

Not shown on 1h–24h windows.

---

## What Does NOT Change

- Night bands, threshold lines, ruler plugin, fullscreen, tooltip, theme toggle — all unchanged.
- The `updateWindowLabels` calls for stat widget sub-labels (high, low, avg, etc.) — unchanged.
- localStorage key (`dexmon-window`) — unchanged. New windows 1h/3h/6h are valid stored values.
- All alarms tab functionality — unchanged.

---

## Acceptance Criteria

- [ ] Dropdown replaces pills; shows Short/Long groups; active window has checkmark; closes on outside click.
- [ ] 1h, 3h, 6h, 12h, 24h windows available and functional end-to-end.
- [ ] X-axis: start and end always labeled; intermediate ticks per table above; no overlapping labels on any window.
- [ ] Dots on 1h and 3h only.
- [ ] Trend line on 7d, 30d, 90d.
- [ ] No desktop/mobile layout regressions.
