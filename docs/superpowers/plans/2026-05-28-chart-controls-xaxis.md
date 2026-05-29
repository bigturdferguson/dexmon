# Chart Controls & X-Axis Labels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the five pill-button time-range selector with a grouped dropdown (Short: 1h/3h/6h/12h, Long: 24h/7d/30d/90d), fix the x-axis so every window shows non-overlapping start/end/intermediate labels, and add dots-only-on-short and a trend line on 7d+.

**Architecture:** All changes are in two files: `dashboard/handler.go` (add three window cases) and `dashboard/static/index.html` (CSS, HTML, and JS). No new files. The label-builder becomes a function that receives the full display-readings array so it can always label the first/last point and apply suppression zones; long windows snap explicit tick timestamps to the nearest data-point index instead of relying on exact timestamp matches (which LTTB breaks).

**Tech Stack:** Vanilla HTML/CSS/JS inside a single-file dashboard · Chart.js 4.4.4 · Go backend

---

### Reference: key lines in `dashboard/static/index.html` as of this plan

| Thing | Lines |
|---|---|
| `.window-pills` / `.pill` CSS | 282–297 |
| `.window-pills` HTML div | 381–387 |
| `updateWindowLabels` | 573–582 |
| `setWindow` | 584–589 |
| `chartLabel` | 610–633 |
| `isMinorTick` | 635–641 |
| `rulerPlugin` | 643–665 |
| `lttb` | 828–865 |
| `prepareChartData` | 867–870 |
| `renderChart` | 872–978 |

---

### Task 1: Backend — add 1h, 3h, 6h windows

**Files:**
- Modify: `dashboard/handler.go:174-187`

- [ ] **Step 1: Add the three new cases to `windowDuration`**

Replace the entire `windowDuration` function body:

```go
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
```

- [ ] **Step 2: Run tests**

```bash
go test ./...
```

Expected: all packages pass, no failures.

- [ ] **Step 3: Commit**

```bash
git add dashboard/handler.go
git commit -m "feat(chart): add 1h, 3h, 6h window support to backend"
```

---

### Task 2: CSS — replace pill styles with dropdown styles

**Files:**
- Modify: `dashboard/static/index.html:282-297`

- [ ] **Step 1: Replace the pill CSS block**

Find and replace lines 282–297 (`.window-pills` through end of `.pill.active`):

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

- [ ] **Step 2: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(chart): replace pill CSS with window-select dropdown styles"
```

---

### Task 3: HTML — replace pill buttons with dropdown markup

**Files:**
- Modify: `dashboard/static/index.html:381-387`

- [ ] **Step 1: Replace the `.window-pills` div**

Find:
```html
          <div class="window-pills">
            <button class="pill" data-window="12h" onclick="setWindow('12h')">12h</button>
            <button class="pill" data-window="24h" onclick="setWindow('24h')">24h</button>
            <button class="pill" data-window="7d"  onclick="setWindow('7d')">7d</button>
            <button class="pill" data-window="30d" onclick="setWindow('30d')">30d</button>
            <button class="pill" data-window="90d" onclick="setWindow('90d')">90d</button>
          </div>
```

Replace with:
```html
          <div class="wsel-wrap" id="window-select">
            <button class="wsel-btn" id="wsel-btn" onclick="toggleWindowSelect()">
              <span id="wsel-label">24h</span>
              <span class="wsel-chevron">▼</span>
            </button>
            <div class="wsel-menu">
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

- [ ] **Step 2: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(chart): replace pill buttons with grouped window-select dropdown"
```

---

### Task 4: JS — wire up the dropdown

**Files:**
- Modify: `dashboard/static/index.html` — functions `updateWindowLabels` (line 573) and `setWindow` (line 584), plus adding `toggleWindowSelect` and a document click listener

- [ ] **Step 1: Replace `updateWindowLabels`**

Find the entire `updateWindowLabels` function:
```js
    function updateWindowLabels(w) {
      ['high', 'low', 'avg', 'stddev', 'cv', 'tir', 'quartiles'].forEach(id => {
        const el = document.getElementById('sub-' + id);
        if (el) el.textContent = w;
      });
      document.getElementById('chart-title').textContent = chartTitle(w);
      document.querySelectorAll('.pill').forEach(b => {
        b.classList.toggle('active', b.dataset.window === w);
      });
    }
```

Replace with:
```js
    function updateWindowLabels(w) {
      ['high', 'low', 'avg', 'stddev', 'cv', 'tir', 'quartiles'].forEach(id => {
        const el = document.getElementById('sub-' + id);
        if (el) el.textContent = w;
      });
      document.getElementById('chart-title').textContent = chartTitle(w);
      document.getElementById('wsel-label').textContent = w;
      document.querySelectorAll('.wsel-item').forEach(b => {
        b.classList.toggle('active', b.dataset.window === w);
      });
    }
```

- [ ] **Step 2: Replace `setWindow` and add `toggleWindowSelect` + click-outside listener**

Find:
```js
    function setWindow(w) {
      currentWindow = w;
      localStorage.setItem('dexmon-window', w);
      if (chart) { chart.destroy(); chart = null; }
      fetchAndRender();
    }
```

Replace with:
```js
    function toggleWindowSelect() {
      document.getElementById('window-select').classList.toggle('open');
    }

    document.addEventListener('click', function(e) {
      const ws = document.getElementById('window-select');
      if (ws && !ws.contains(e.target)) ws.classList.remove('open');
    });

    function setWindow(w) {
      document.getElementById('window-select').classList.remove('open');
      currentWindow = w;
      localStorage.setItem('dexmon-window', w);
      if (chart) { chart.destroy(); chart = null; }
      fetchAndRender();
    }
```

- [ ] **Step 3: Verify in browser**

Open the dashboard. The dropdown button should show "24h" (or whatever was last saved). Clicking opens the menu showing Short / Long groups. Selecting a window closes the menu and updates the chart. Clicking outside the menu also closes it.

- [ ] **Step 4: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(chart): wire up window-select dropdown toggle and active state"
```

---

### Task 5: JS — `buildLabels` (replaces `chartLabel`)

**Files:**
- Modify: `dashboard/static/index.html` — replace `chartLabel` (lines 610–633) with `buildLabels` + three helper functions; update `renderChart` labels line (line 878)

Context: `chartLabel` is called per-point and cannot see the full dataset, so it can't label the first/last point or apply suppression. The replacement `buildLabels(displayReadings, w)` receives the full array and returns the complete labels array.

- [ ] **Step 1: Replace `chartLabel` with the new functions**

Find and remove the entire `chartLabel` function (lines 610–633):
```js
    function chartLabel(isoStr, w) {
      const dt = new Date(isoStr);
      if (w === '12h' || w === '24h') {
        return dt.getMinutes() === 0
          ? dt.getHours().toString().padStart(2, '0') + ':00'
          : '';
      }
      if (w === '7d') {
        return (dt.getHours() === 0 && dt.getMinutes() === 0)
          ? dt.toLocaleDateString([], { weekday: 'short' })
          : '';
      }
      if (w === '30d') {
        return (dt.getHours() === 0 && dt.getMinutes() === 0 && dt.getDate() % 5 === 1)
          ? dt.toLocaleDateString([], { month: 'short', day: 'numeric' })
          : '';
      }
      if (w === '90d') {
        return (dt.getHours() === 0 && dt.getMinutes() === 0 && dt.getDate() === 1)
          ? dt.toLocaleDateString([], { month: 'short' })
          : '';
      }
      return '';
    }
```

Replace with:
```js
    // Minimum gap (ms) from start/end before an intermediate tick is shown.
    // Set to half the tick interval so ticks never crowd the endpoint labels.
    const LABEL_GAP_MS = {
      '1h':  7.5 * 60 * 1000,
      '3h':  15  * 60 * 1000,
      '6h':  30  * 60 * 1000,
      '12h': 60  * 60 * 1000,
      '24h': 2   * 60 * 60 * 1000,
      '7d':  24  * 60 * 60 * 1000,
      '30d': 4   * 24 * 60 * 60 * 1000,
      '90d': 16  * 24 * 60 * 60 * 1000,
    };
    const LONG_WINDOWS = new Set(['7d', '30d', '90d']);

    function fmtT(d) {
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }
    function fmtD(d) {
      return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
    }
    function fmtM(d) {
      return d.toLocaleDateString([], { month: 'short' });
    }

    // Generate explicit tick timestamps for long windows.
    function longTickMs(w, startMs, endMs) {
      const ticks = [];
      const d = new Date(startMs);
      if (w === '7d') {
        d.setHours(0, 0, 0, 0); d.setDate(d.getDate() + 1);
        while (d.getTime() <= endMs) { ticks.push(d.getTime()); d.setDate(d.getDate() + 1); }
      } else if (w === '30d') {
        d.setHours(0, 0, 0, 0); d.setDate(d.getDate() + 1);
        while (d.getTime() <= endMs) { ticks.push(d.getTime()); d.setDate(d.getDate() + 7); }
      } else if (w === '90d') {
        d.setHours(0, 0, 0, 0); d.setDate(1); d.setMonth(d.getMonth() + 1);
        while (d.getTime() <= endMs) { ticks.push(d.getTime()); d.setMonth(d.getMonth() + 1); }
      }
      return ticks;
    }

    // Find the index of the reading in rds whose timestamp is closest to targetMs.
    function nearestReadingIdx(rds, targetMs) {
      let best = 0, bestDiff = Infinity;
      for (let i = 0; i < rds.length; i++) {
        const diff = Math.abs(new Date(rds[i].recorded_at).getTime() - targetMs);
        if (diff < bestDiff) { bestDiff = diff; best = i; }
        else if (diff > bestDiff) break;
      }
      return best;
    }

    function buildLabels(displayReadings, w) {
      const n = displayReadings.length;
      const startMs = new Date(displayReadings[0].recorded_at).getTime();
      const endMs   = new Date(displayReadings[n - 1].recorded_at).getTime();
      const gap     = LABEL_GAP_MS[w] || 0;

      // ── Short windows: iterate points, apply suppression ─────────────────
      if (!LONG_WINDOWS.has(w)) {
        return displayReadings.map((r, i) => {
          const dt  = new Date(r.recorded_at);
          const ts  = dt.getTime();
          const min = dt.getMinutes();
          const h   = dt.getHours();
          if (i === 0 || i === n - 1) return fmtT(dt);
          if (ts - startMs < gap || endMs - ts < gap) return '';
          if (w === '1h')  return min % 15 === 0 ? fmtT(dt) : '';
          if (w === '3h')  return (min === 0 || min === 30) ? fmtT(dt) : '';
          if (w === '6h')  return min === 0 ? fmtT(dt) : '';
          if (w === '12h') return (min === 0 && h % 2 === 0) ? fmtT(dt) : '';
          if (w === '24h') return (min === 0 && h % 4 === 0) ? fmtT(dt) : '';
          return '';
        });
      }

      // ── Long windows: snap explicit tick timestamps to nearest point ──────
      const labels  = new Array(n).fill('');
      const fmt     = w === '90d' ? fmtM : fmtD;
      labels[0]     = fmt(new Date(startMs));
      labels[n - 1] = fmt(new Date(endMs));

      const used = new Set([0, n - 1]);
      for (const tickMs of longTickMs(w, startMs, endMs)) {
        if (tickMs - startMs < gap || endMs - tickMs < gap) continue;
        const idx = nearestReadingIdx(displayReadings, tickMs);
        if (used.has(idx)) continue;
        used.add(idx);
        labels[idx] = fmt(new Date(tickMs));
      }
      return labels;
    }
```

- [ ] **Step 2: Update `renderChart` to call `buildLabels`**

Find (line 878):
```js
      const labels    = displayReadings.map(r => chartLabel(r.recorded_at, currentWindow));
```

Replace with:
```js
      const labels    = buildLabels(displayReadings, currentWindow);
```

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 4: Verify in browser**

Open the dashboard and check every window:
- 1h: start time, ticks at :15/:30/:45, end time, no overlap
- 3h: start time, ticks at :00 and :30, end time
- 6h: start time, ticks at every :00, end time
- 12h: start time, ticks every 2h, end time
- 24h: start time, ticks every 4h, end time
- 7d: start date, one label per calendar day (suppressed if within 24h of endpoints), end date
- 30d: start date, weekly labels, end date
- 90d: start month, monthly labels, end month

- [ ] **Step 5: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(chart): replace chartLabel with buildLabels — start/end always shown, suppression zones, long windows snap to nearest point"
```

---

### Task 6: JS — update `isMinorTick` for new windows

**Files:**
- Modify: `dashboard/static/index.html:635-641`

- [ ] **Step 1: Replace `isMinorTick`**

Find:
```js
    function isMinorTick(isoStr, w) {
      const dt = new Date(isoStr);
      if (w === '12h' || w === '24h') return dt.getMinutes() === 30;
      if (w === '90d') return dt.getHours() === 0 && dt.getMinutes() === 0 && dt.getDate() === 15;
      // 7d and 30d: noon mark
      return dt.getHours() === 12 && dt.getMinutes() === 0;
    }
```

Replace with:
```js
    function isMinorTick(isoStr, w) {
      const dt  = new Date(isoStr);
      const min = dt.getMinutes();
      if (w === '1h')  return min % 5 === 0 && min % 15 !== 0;
      if (w === '3h')  return min === 15 || min === 45;
      if (w === '6h' || w === '12h' || w === '24h') return min === 30;
      return false; // no minor ticks on 7d / 30d / 90d
    }
```

- [ ] **Step 2: Verify in browser**

Switch to 1h: small tick marks every 5 min between the 15-min labeled ticks. Switch to 3h: tick marks at :15 and :45. Switch to 24h: tick at every :30. Switch to 7d: no minor ticks.

- [ ] **Step 3: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(chart): update isMinorTick for 1h/3h/6h, remove minor ticks on long windows"
```

---

### Task 7: JS — dots, sampling, and trend line

**Files:**
- Modify: `dashboard/static/index.html` — `prepareChartData` (line 867), `renderChart` (lines 872–978)

- [ ] **Step 1: Update `prepareChartData`**

Find:
```js
    function prepareChartData(readings, w) {
      if (w === '12h' || readings.length <= 200) return readings;
      return lttb(readings, 200);
    }
```

Replace with:
```js
    const SHORT_WINDOWS = new Set(['1h', '3h', '6h', '12h', '24h']);

    function prepareChartData(readings, w) {
      if (SHORT_WINDOWS.has(w) || readings.length <= 200) return readings;
      return lttb(readings, 200);
    }

    function movingAverage(readings, windowPct) {
      const k = Math.max(3, Math.round(readings.length * windowPct));
      return readings.map((_, i) => {
        const lo = Math.max(0, i - Math.floor(k / 2));
        const hi = Math.min(readings.length, lo + k);
        let sum = 0;
        for (let j = lo; j < hi; j++) sum += readings[j].value;
        return Math.round(sum / (hi - lo));
      });
    }
```

- [ ] **Step 2: Update dot logic and add trend line in `renderChart`**

Find in `renderChart` (just after `prepareChartData` call):
```js
      const showDots = currentWindow === '12h' || currentWindow === '24h';
      const pointRadius = showDots ? (displayReadings.length > 100 ? 2 : 3) : 0;
```

Replace with:
```js
      const showDots    = currentWindow === '1h' || currentWindow === '3h';
      const pointRadius = showDots ? (displayReadings.length > 60 ? 2 : 3) : 0;
      const showTrend   = LONG_WINDOWS.has(currentWindow);
      const trendData   = showTrend ? movingAverage(displayReadings, 0.12) : [];
```

- [ ] **Step 3: Add trend dataset to the datasets array (chart-create path)**

Find the `datasets` array in the `new Chart(...)` call. It currently has 3 entries. Add the trend dataset after the `highLine` entry:

```js
            {
              data: highLine,
              borderColor: cssVar('--high-line'),
              borderWidth: 1,
              borderDash: [5, 4],
              pointRadius: 0,
              fill: false,
            },
```

Replace the closing `],` of the datasets array with:
```js
            {
              data: highLine,
              borderColor: cssVar('--high-line'),
              borderWidth: 1,
              borderDash: [5, 4],
              pointRadius: 0,
              fill: false,
            },
            ...(showTrend ? [{
              data: trendData,
              borderColor: 'rgba(99,102,241,0.5)',
              borderWidth: 2.5,
              pointRadius: 0,
              tension: 0.4,
              fill: false,
            }] : []),
```

- [ ] **Step 4: Update the chart-update path to replace datasets entirely**

The existing update path (inside `if (chart) { ... return; }`) sets individual dataset properties. Replace the entire update block:

Find:
```js
      if (chart) {
        chart._rulerWindow   = currentWindow;
        chart._rulerReadings = displayReadings;
        chart._rawReadings   = readings;
        chart.data.labels = labels;
        chart.data.datasets[0].data = values;
        chart.data.datasets[0].pointBackgroundColor = ptColors;
        chart.data.datasets[0].pointRadius = pointRadius;
        chart.data.datasets[0].pointHoverRadius = 4;
        chart.data.datasets[1].data = lowLine;
        chart.data.datasets[2].data = highLine;
        chart.update('none');
        return;
      }
```

Replace with:
```js
      if (chart) {
        chart._rulerWindow   = currentWindow;
        chart._rulerReadings = displayReadings;
        chart._rawReadings   = readings;
        chart.data.labels    = labels;
        chart.data.datasets  = [
          {
            data: values,
            borderColor: '#6366f1',
            borderWidth: 2,
            pointBackgroundColor: ptColors,
            pointRadius,
            pointHoverRadius: 4,
            tension: 0.3,
            fill: false,
          },
          {
            data: lowLine,
            borderColor: cssVar('--low-line'),
            borderWidth: 1,
            borderDash: [5, 4],
            pointRadius: 0,
            fill: '+1',
            backgroundColor: 'rgba(22,163,74,0.08)',
          },
          {
            data: highLine,
            borderColor: cssVar('--high-line'),
            borderWidth: 1,
            borderDash: [5, 4],
            pointRadius: 0,
            fill: false,
          },
          ...(showTrend ? [{
            data: trendData,
            borderColor: 'rgba(99,102,241,0.5)',
            borderWidth: 2.5,
            pointRadius: 0,
            tension: 0.4,
            fill: false,
          }] : []),
        ];
        chart.update('none');
        return;
      }
```

- [ ] **Step 5: Run tests**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Verify in browser**

- 1h and 3h: coloured dots visible on the BG line
- 6h, 12h, 24h: no dots
- 7d, 30d, 90d: smooth trend line (semi-transparent indigo) overlaid on the data line
- Switching windows: trend line appears/disappears correctly

- [ ] **Step 7: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(chart): dots on 1h/3h only; LTTB for 6h+; moving-average trend line on 7d/30d/90d"
```

---

## Acceptance Checklist

After all tasks complete, verify:

- [ ] Dropdown shows Short (1h/3h/6h/12h) and Long (24h/7d/30d/90d) groups
- [ ] Active window shows checkmark; selecting a window updates label and chart
- [ ] Click outside closes the dropdown
- [ ] 1h–24h: start/end labeled, intermediate ticks per spec, no overlapping labels
- [ ] 7d: one label per day (suppressed within 24h of endpoints)
- [ ] 30d: weekly labels (suppressed within 4 days of endpoints)
- [ ] 90d: monthly labels (suppressed within 16 days of endpoints)
- [ ] Dots on 1h and 3h only
- [ ] Trend line on 7d, 30d, 90d
- [ ] `go test ./...` passes
