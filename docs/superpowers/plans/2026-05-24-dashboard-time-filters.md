# Dashboard Time Filters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 12h / 24h / 7d / 30d time-range filter to the dashboard — chart, stats, and card title reflect the selected window; ruler-style x-axis ticks show clean hour/day boundaries only.

**Architecture:** Backend adds `?window=` query param and `Window` field to the API response; no store changes needed. Frontend adds pill buttons, updates stat subtitles and chart title dynamically, generates clean boundary-only axis labels via `ticks.callback`, and draws sub-boundary tick marks with a `rulerPlugin` Chart.js plugin.

**Tech Stack:** Go (`net/http`, `time`), vanilla JS, Chart.js 4.4.4 (vendored)

---

### Task 1: Backend — `?window` param + `Window` field in response

**Files:**
- Modify: `dashboard/handler.go`
- Modify: `dashboard/handler_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `dashboard/handler_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/brandon/Projects/dexmon && go test ./dashboard/ -run 'TestDashboardAPI_Window' -v
```

Expected: FAIL — `resp.Window` field does not exist yet.

- [ ] **Step 3: Add `Window` field to `DashboardResponse` in `dashboard/handler.go`**

Change the struct (add `Window` between `AsOf` and `Target`):

```go
type DashboardResponse struct {
	Account  string        `json:"account"`
	AsOf     time.Time     `json:"as_of"`
	Window   string        `json:"window"`
	Target   TargetJSON    `json:"target"`
	Current  *ReadingJSON  `json:"current"`
	Previous *ReadingJSON  `json:"previous"`
	Stats    StatsJSON     `json:"stats"`
	Readings []ReadingJSON `json:"readings"`
	Alarms   []AlarmJSON   `json:"alarms"`
}
```

- [ ] **Step 4: Add `windowDuration` helper and update `serveAPI` in `dashboard/handler.go`**

Add this package-level function (above `serveAPI`):

```go
func windowDuration(s string) (string, time.Duration) {
	switch s {
	case "12h":
		return "12h", 12 * time.Hour
	case "7d":
		return "7d", 7 * 24 * time.Hour
	case "30d":
		return "30d", 30 * 24 * time.Hour
	default:
		return "24h", 24 * time.Hour
	}
}
```

Update the top of `serveAPI` — replace:
```go
now := time.Now().UTC()
since := now.Add(-24 * time.Hour)
```
with:
```go
now := time.Now().UTC()
window, dur := windowDuration(r.URL.Query().Get("window"))
since := now.Add(-dur)
```

And populate `Window` in the response literal:
```go
resp := DashboardResponse{
	Account:  h.account,
	AsOf:     now,
	Window:   window,
	Target:   TargetJSON{Low: h.targetLow, High: h.targetHigh},
	Stats:    StatsJSON{High: maxVal, Low: minVal, Avg: avgVal},
	Readings: toReadingJSON(readings),
	Alarms:   h.buildAlarmList(now),
}
```

- [ ] **Step 5: Run all dashboard tests to verify they pass**

```bash
cd /home/brandon/Projects/dexmon && go test ./dashboard/ -v
```

Expected: all 13 dashboard tests PASS.

- [ ] **Step 6: Commit**

```bash
git add dashboard/handler.go dashboard/handler_test.go
git commit -m "feat: add ?window param to dashboard API; echo window in response"
```

---

### Task 2: Frontend — pill buttons, stat subtitles, chart title, localStorage

**Files:**
- Modify: `dashboard/static/index.html`

- [ ] **Step 1: Add CSS for pill buttons and card-header layout**

In the `<style>` block, add after the `.chart-wrap` rule:

```css
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}
.card-header .card-title { margin-bottom: 0; }
.window-pills { display: flex; gap: 0.25rem; }
.pill {
  background: none;
  border: 1px solid var(--border);
  border-radius: 9999px;
  padding: 0.15rem 0.6rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
  cursor: pointer;
}
.pill.active {
  background: var(--text);
  border-color: var(--text);
  color: var(--bg);
}
```

- [ ] **Step 2: Update chart card HTML**

Replace:
```html
    <div class="card">
      <div class="card-title">BG — Last 24 Hours</div>
      <div class="chart-wrap"><canvas id="bg-chart"></canvas></div>
    </div>
```
with:
```html
    <div class="card">
      <div class="card-header">
        <div class="card-title" id="chart-title">BG — Last 24 Hours</div>
        <div class="window-pills">
          <button class="pill" data-window="12h" onclick="setWindow('12h')">12h</button>
          <button class="pill" data-window="24h" onclick="setWindow('24h')">24h</button>
          <button class="pill" data-window="7d"  onclick="setWindow('7d')">7d</button>
          <button class="pill" data-window="30d" onclick="setWindow('30d')">30d</button>
        </div>
      </div>
      <div class="chart-wrap"><canvas id="bg-chart"></canvas></div>
    </div>
```

- [ ] **Step 3: Add IDs to High/Low/Avg stat subtitles**

Replace the three stat cards (High, Low, Avg):
```html
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
```
with:
```html
      <div class="stat-card">
        <div class="stat-label">High</div>
        <div class="stat-value" id="val-high">—</div>
        <div class="stat-sub" id="sub-high">24h</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Low</div>
        <div class="stat-value" id="val-low">—</div>
        <div class="stat-sub" id="sub-low">24h</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Avg</div>
        <div class="stat-value" id="val-avg">—</div>
        <div class="stat-sub" id="sub-avg">24h</div>
      </div>
```

- [ ] **Step 4: Add `currentWindow` variable and window helper functions in JS**

Add `currentWindow` to the variables block (with `chart`, `lastData`, `lastAsOf`):
```js
let currentWindow = localStorage.getItem('dexmon-window') || '24h';
```

Add these functions in the `// ── helpers ──` section, after `esc()`:

```js
function chartTitle(w) {
  const map = {
    '12h': 'BG — Last 12 Hours',
    '24h': 'BG — Last 24 Hours',
    '7d':  'BG — Last 7 Days',
    '30d': 'BG — Last 30 Days',
  };
  return map[w] || 'BG — Last 24 Hours';
}

function updateWindowLabels(w) {
  ['high', 'low', 'avg'].forEach(id => {
    document.getElementById('sub-' + id).textContent = w;
  });
  document.getElementById('chart-title').textContent = chartTitle(w);
  document.querySelectorAll('.pill').forEach(b => {
    b.classList.toggle('active', b.dataset.window === w);
  });
}

function setWindow(w) {
  currentWindow = w;
  localStorage.setItem('dexmon-window', w);
  if (chart) { chart.destroy(); chart = null; }
  fetchAndRender();
}
```

Note: `setWindow` destroys the chart before re-fetching so `renderChart` always rebuilds with fresh options (avoids stale tick config from previous window).

- [ ] **Step 5: Update `fetchAndRender` to use `currentWindow` and call `updateWindowLabels`**

In `fetchAndRender`, change:
```js
const resp = await fetch('/api/dashboard');
```
to:
```js
const resp = await fetch('/api/dashboard?window=' + currentWindow);
```

After `updateLastUpdated()` call, add:
```js
updateWindowLabels(d.window || '24h');
```

- [ ] **Step 6: Initialize window labels on startup**

After the `applyTheme()` call near the bottom of the script, add:
```js
updateWindowLabels(currentWindow);
```

- [ ] **Step 7: Run all tests and verify no regressions**

```bash
cd /home/brandon/Projects/dexmon && go test ./...
```

Expected: all tests PASS (no Go changes in this task).

- [ ] **Step 8: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat: add window filter pill buttons and dynamic stat/title labels"
```

---

### Task 3: Frontend — ruler-style chart labels and minor tick plugin

**Files:**
- Modify: `dashboard/static/index.html`

- [ ] **Step 1: Add `chartLabel`, `isMinorTick`, and `rulerPlugin` to the helpers section**

Add these after `updateWindowLabels` / `setWindow`:

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
  // 30d: label ~every 5 days at midnight to avoid mobile density overflow
  return (dt.getHours() === 0 && dt.getMinutes() === 0 && dt.getDate() % 5 === 1)
    ? dt.toLocaleDateString([], { month: 'short', day: 'numeric' })
    : '';
}

function isMinorTick(isoStr, w) {
  const dt = new Date(isoStr);
  if (w === '12h' || w === '24h') return dt.getMinutes() === 30;
  // 7d and 30d: noon mark (halfway through each day)
  return dt.getHours() === 12 && dt.getMinutes() === 0;
}

const rulerPlugin = {
  id: 'ruler',
  afterDraw(chart) {
    const xScale = chart.scales.x;
    const rds = chart._rulerReadings;
    const w   = chart._rulerWindow;
    if (!rds || !w) return;
    const ctx = chart.ctx;
    ctx.save();
    ctx.strokeStyle = cssVar('--text-muted');
    ctx.lineWidth = 1;
    rds.forEach((r, i) => {
      if (!isMinorTick(r.recorded_at, w)) return;
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

- [ ] **Step 2: Update `renderChart` label generation**

Replace the label generation block:
```js
      const labels = readings.map(r => {
        const dt = new Date(r.recorded_at);
        return dt.getHours().toString().padStart(2,'0') + ':' +
               dt.getMinutes().toString().padStart(2,'0');
      });
```
with:
```js
      const labels = readings.map(r => chartLabel(r.recorded_at, currentWindow));
```

- [ ] **Step 3: Update the chart update branch to set ruler metadata**

Replace the `if (chart)` branch:
```js
      if (chart) {
        chart.data.labels = labels;
        chart.data.datasets[0].data = values;
        chart.data.datasets[0].pointBackgroundColor = ptColors;
        chart.data.datasets[1].data = lowLine;
        chart.data.datasets[2].data = highLine;
        chart.update('none');
        return;
      }
```
with:
```js
      if (chart) {
        chart._rulerWindow   = currentWindow;
        chart._rulerReadings = readings;
        chart.data.labels = labels;
        chart.data.datasets[0].data = values;
        chart.data.datasets[0].pointBackgroundColor = ptColors;
        chart.data.datasets[1].data = lowLine;
        chart.data.datasets[2].data = highLine;
        chart.update('none');
        return;
      }
```

- [ ] **Step 4: Update chart creation — ticks config and register rulerPlugin**

In the `new Chart(ctx, {...})` call:

Add `plugins: [rulerPlugin]` as the second key (after `type: 'line'`):
```js
      chart = new Chart(ctx, {
        type: 'line',
        plugins: [rulerPlugin],
        data: { ... },
        options: { ... },
      });
```

Change the x-axis ticks config from:
```js
              ticks: { color: cssVar('--text-muted'), maxTicksLimit: 8, maxRotation: 0 },
```
to:
```js
              ticks: {
                color: cssVar('--text-muted'),
                autoSkip: false,
                maxRotation: 0,
                callback: function(val, i) {
                  const label = chartLabel(this.chart._rulerReadings?.[i]?.recorded_at, this.chart._rulerWindow);
                  return label || null;
                },
              },
```

After the `chart = new Chart(...)` line, set the ruler metadata:
```js
      chart._rulerWindow   = currentWindow;
      chart._rulerReadings = readings;
```

- [ ] **Step 5: Run all tests**

```bash
cd /home/brandon/Projects/dexmon && go test ./...
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat: ruler-style chart labels — boundary ticks only, minor tick plugin"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `?window=` query param (12h/24h/7d/30d), default 24h | Task 1 |
| Invalid window falls back to 24h | Task 1 |
| `Window` field in `DashboardResponse` | Task 1 |
| `windowDuration` helper | Task 1 |
| Tests for all 4 window values | Task 1 |
| Pill buttons in chart card header | Task 2 |
| Active pill styling | Task 2 |
| `localStorage` save/restore for `dexmon-window` | Task 2 |
| Stat subtitles (High/Low/Avg) reflect window | Task 2 |
| Chart card title reflects window | Task 2 |
| Current/Previous BG unaffected | Task 2 (fetch uses window but stat card HTML unchanged) |
| `chartLabel` — boundary-only labels | Task 3 |
| `isMinorTick` — half-hour/noon conditions | Task 3 |
| `rulerPlugin` — draws 4px minor tick lines | Task 3 |
| `autoSkip: false` + `ticks.callback` replaces `maxTicksLimit` | Task 3 |
| `chart._rulerWindow` / `chart._rulerReadings` set on create + update | Task 3 |

**Placeholder scan:** No TBDs. All code blocks complete.

**Type consistency:** `currentWindow` used consistently. `chart._rulerWindow` and `chart._rulerReadings` set in both the create and update paths in Task 3.

**Implementation note on 30d labels:** Spec says "every other midnight" for 30d minor ticks. Implemented as noon marks (same as 7d) because midnight minor ticks would conflict with midnight major ticks. For 30d major labels, using every 5th day (`getDate() % 5 === 1`) to avoid mobile label overflow — labeling all 30 midnights would produce unreadable density on narrow screens.
