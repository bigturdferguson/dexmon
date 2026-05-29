# Desktop Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the desktop dashboard into a two-column layout with a hero BG card, distribution strip, TIR donut, alarms flyout, and health checks widget — without changing the mobile layout.

**Architecture:** Backend gains a `meta` KV table (watchdog ping tracking) and new `health`/`time_below_range`/`time_above_range` fields in the API. Frontend restructures desktop CSS/HTML into a `1fr 300px` grid with new components; all mobile-specific code in the existing `@media (max-width: 640px)` block is untouched.

**Tech Stack:** Go (SQLite/store), Chart.js (donut + line), vanilla JS/CSS

---

### Task 1: Add meta KV table to store

**Files:**
- Modify: `store/schema.go`
- Create: `store/meta.go`
- Modify: `store/store_test.go`

- [ ] Add `meta` table to schema

In `store/schema.go`, append to the `schema` const (before the final backtick):

```go
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`
```

- [ ] Create `store/meta.go`

```go
package store

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

func (s *Store) GetMeta(key string) (value string, ok bool, err error) {
	err = s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}
```

- [ ] Write tests in `store/store_test.go` — append:

```go
func TestSetMeta_GetMeta(t *testing.T) {
	s := newTestStore(t)
	ok, err := func() (bool, error) {
		_, ok, err := s.GetMeta("foo")
		return ok, err
	}()
	if err != nil { t.Fatalf("GetMeta empty: %v", err) }
	if ok { t.Error("expected not found before set") }

	if err := s.SetMeta("foo", "bar"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	val, ok, err := s.GetMeta("foo")
	if err != nil { t.Fatalf("GetMeta after set: %v", err) }
	if !ok { t.Error("expected found after set") }
	if val != "bar" { t.Errorf("got %q want %q", val, "bar") }

	// Upsert
	if err := s.SetMeta("foo", "baz"); err != nil {
		t.Fatalf("SetMeta upsert: %v", err)
	}
	val, _, _ = s.GetMeta("foo")
	if val != "baz" { t.Errorf("upsert: got %q want %q", val, "baz") }
}
```

- [ ] Run tests: `cd /home/brandon/Projects/dexmon && go test ./store/... -v -run TestSetMeta` — expect PASS

- [ ] Commit:
```bash
git add store/schema.go store/meta.go store/store_test.go
git commit -m "feat(store): add meta KV table with SetMeta/GetMeta"
```

---

### Task 2: Add TBR/TAR stats + health field to API

**Files:**
- Modify: `dashboard/handler.go`
- Modify: `dashboard/stats_test.go`

- [ ] Add `TimeBelowRange`/`TimeAboveRange` to `StatsJSON` and `computeStats` in `handler.go`:

Replace the `StatsJSON` struct:
```go
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
```

In `computeStats`, replace `inRange := 0` with `inRange, belowRange, aboveRange := 0, 0, 0`, add the below/above increments in the loop, compute `tbr`/`tar`, and return them.

Full updated `computeStats`:
```go
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
		if v < minVal { minVal = v }
		if v > maxVal { maxVal = v }
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
	if variance < 0 { variance = 0 }
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
```

- [ ] Add `HealthJSON` and `health` field to `DashboardResponse`:

```go
type WatchdogHealthJSON struct {
	Configured bool    `json:"configured"`
	LastPingAt *string `json:"last_ping_at,omitempty"`
}

type HealthJSON struct {
	Watchdog WatchdogHealthJSON `json:"watchdog"`
}
```

Add `Health HealthJSON` to `DashboardResponse`.

- [ ] Add `watchdogURL string` field to `Handler` struct; update `New` signature:

```go
func New(st *store.Store, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int, watchdogURL string) *Handler {
	return &Handler{store: st, account: account, alarms: alarms, recipients: recipients, targetLow: targetLow, targetHigh: targetHigh, watchdogURL: watchdogURL}
}
```

- [ ] Compute `health` in `serveAPI` and add to response (before `json.NewEncoder` call):

```go
health := HealthJSON{
	Watchdog: WatchdogHealthJSON{Configured: h.watchdogURL != ""},
}
if h.watchdogURL != "" {
	if v, ok, err := h.store.GetMeta("last_watchdog_ping"); ok && err == nil {
		health.Watchdog.LastPingAt = &v
	}
}
resp.Health = health
```

- [ ] Update `dashboard/stats_test.go` to test TBR/TAR (add assertions to existing tests for the new fields — TBR=0/TAR=0 for in-range data, TBR=100 for all-below, etc.)

- [ ] Run: `go test ./dashboard/... -v` — expect PASS

- [ ] Commit:
```bash
git add dashboard/handler.go dashboard/stats_test.go
git commit -m "feat(api): add time_below_range, time_above_range, and health fields"
```

---

### Task 3: Thread watchdog URL through callback and record ping in poller

**Files:**
- Modify: `callback/server.go`
- Modify: `main.go`
- Modify: `poller/poller.go`

- [ ] Update `callback.New` to accept and forward `watchdogURL`:

```go
func New(st *store.Store, port int, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int, watchdogURL string) *Server {
	s := &Server{store: st, port: port, mux: http.NewServeMux()}
	dash := dashboard.New(st, account, alarms, recipients, targetLow, targetHigh, watchdogURL)
	// ... rest unchanged
```

- [ ] Update `main.go` to pass `cfg.Health.Watchdog.PingURL`:

```go
srv := callback.New(st, cfg.Server.CallbackPort, accountName, accountAlarms, cfg.Recipients, targetLow, targetHigh, cfg.Health.Watchdog.PingURL)
```

- [ ] In `poller/poller.go`, after `health.PingWatchdog(url)` succeeds, store the ping time:

```go
if url := p.healthCfg.Watchdog.PingURL; url != "" {
	health.PingWatchdog(url)
	if err := p.store.SetMeta("last_watchdog_ping", now.Format(time.RFC3339)); err != nil {
		log.Printf("[%s] set last_watchdog_ping: %v", p.accountName, err)
	}
}
```

- [ ] Run: `go build ./...` — expect no errors

- [ ] Run: `go test ./... ` — expect PASS

- [ ] Commit:
```bash
git add callback/server.go main.go poller/poller.go
git commit -m "feat: thread watchdog URL to handler; record last ping time in store"
```

---

### Task 4: Restructure desktop HTML layout

**Files:**
- Modify: `dashboard/static/index.html`

This task adds/changes HTML structure only; CSS and JS follow in Tasks 5–7.

- [ ] In the `<head>`, update the Google Fonts link to load Inter on **all** viewports (remove the `media="(max-width: 640px)"` restriction since we'll use Inter on desktop too), or just leave the font load as-is (system font stack is fine for desktop).

- [ ] In `<header>`, move `#window-select` markup into the header (it's currently only inside the chart card). Add a new `id="desktop-wsel-wrap"` wrapper in the header right side, before the theme button:

```html
<header>
  <h1>dexmon</h1>
  <div class="header-right">
    <span id="last-updated">Loading…</span>
    <div class="wsel-wrap" id="header-window-select">
      <!-- existing mobile wsel markup (unchanged) -->
    </div>
    <div class="wsel-wrap" id="desktop-wsel-wrap">
      <button class="wsel-btn" id="desktop-wsel-btn" onclick="toggleDesktopWindowSelect()">
        <span id="desktop-wsel-label">24h</span>
        <span class="wsel-chevron">▼</span>
      </button>
      <div class="wsel-menu" id="desktop-wsel-menu">
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
    <button id="theme-btn" onclick="toggleTheme()">☀︎</button>
  </div>
</header>
```

- [ ] Wrap the existing `<main>` content in a new structure. Inside `<main>`, add a `<div id="desktop-left">` and `<div id="desktop-right">` wrapping the appropriate children. The desktop hero card and right-panel widgets are new elements inserted here. The existing chart card, distribution strip, tab panels, and tab bar remain in place (CSS will reposition them).

Add before the chart `.card`:
```html
<!-- Desktop-only hero card -->
<div id="desktop-hero" class="card">
  <div id="dh-label" class="stat-label">Current BG</div>
  <div id="dh-body">
    <div id="dh-left">
      <div id="dh-val" class="stat-value">—</div>
      <span id="dh-unit" class="dh-unit">mg/dL</span>
    </div>
    <div class="dh-divider"></div>
    <div id="dh-meta">
      <div class="dh-meta-item">
        <div class="dh-meta-label">Change</div>
        <div class="dh-meta-val" id="dh-delta">—</div>
      </div>
      <div class="dh-meta-item">
        <div class="dh-meta-label">Trend</div>
        <div class="dh-meta-val" id="dh-trend">—</div>
      </div>
      <div class="dh-meta-item">
        <div class="dh-meta-label">Updated</div>
        <div class="dh-meta-val dh-meta-age" id="dh-age">—</div>
      </div>
    </div>
  </div>
</div>
```

Add a `<div id="desktop-right">` wrapping three new right-panel widgets (after the chart `.card` sibling):
```html
<div id="desktop-right">
  <!-- TIR Donut -->
  <div class="card" id="desktop-donut-card">
    <div class="stat-label">Time in Range</div>
    <div id="desktop-donut-wrap">
      <canvas id="desktop-donut-canvas"></canvas>
    </div>
    <div id="desktop-tir-legend"></div>
  </div>
  <!-- Alarms widget -->
  <div class="card" id="desktop-alarms-widget" onclick="openAlarmsFlyout()">
    <div class="desktop-widget-header">
      <div class="stat-label" style="margin:0;">Alarms</div>
      <span class="desktop-view-all">View all ›</span>
    </div>
    <div id="desktop-alarms-rows"></div>
  </div>
  <!-- Health checks widget -->
  <div class="card" id="desktop-health-card">
    <div class="stat-label">Health</div>
    <div id="desktop-health-dexcom" class="desktop-health-row">
      <span class="desktop-health-dot" id="dh-dexcom-dot"></span>
      <span class="desktop-health-name">Dexcom API</span>
      <span class="desktop-health-status" id="dh-dexcom-status"></span>
    </div>
    <div class="desktop-health-sub" id="dh-dexcom-sub"></div>
    <div class="desktop-health-divider"></div>
    <div id="desktop-health-watchdog" class="desktop-health-row">
      <span class="desktop-health-dot" id="dh-watchdog-dot"></span>
      <span class="desktop-health-name">Watchdog</span>
      <span class="desktop-health-status" id="dh-watchdog-status"></span>
    </div>
    <div class="desktop-health-sub" id="dh-watchdog-sub"></div>
  </div>
</div>
```

Add flyout HTML at bottom of `<body>` (before closing `</body>`):
```html
<!-- Alarms flyout -->
<div id="alarms-overlay" class="alarms-overlay hidden" onclick="closeAlarmsFlyout()"></div>
<div id="alarms-flyout" class="alarms-flyout hidden">
  <div class="alarms-flyout-header">
    <span class="alarms-flyout-title">Alarms</span>
    <button class="alarms-flyout-close" onclick="closeAlarmsFlyout()">✕</button>
  </div>
  <div class="alarms-flyout-body">
    <div class="alarms-flyout-section">
      <div class="alarms-flyout-section-title">Current Status</div>
      <table class="alarms-table">
        <thead><tr><th>Alarm</th><th>Priority</th><th>Last Fired</th><th>Status</th></tr></thead>
        <tbody id="flyout-alarm-rows"></tbody>
      </table>
    </div>
    <div class="alarms-flyout-section">
      <div class="alarms-flyout-section-title">Recent History</div>
      <table class="alarms-table">
        <thead><tr><th>Alarm</th><th>BG</th><th>Time</th></tr></thead>
        <tbody id="flyout-history-rows"></tbody>
      </table>
    </div>
  </div>
</div>
```

- [ ] Verify the page still loads (run dev server): `go run . -config config.toml` and open browser. Mobile layout should be unchanged.

- [ ] Commit:
```bash
git add dashboard/static/index.html
git commit -m "feat(dashboard): add desktop HTML structure (hero, right panel, flyout)"
```

---

### Task 5: Add desktop CSS

**Files:**
- Modify: `dashboard/static/index.html` (style block)

Add a `@media (min-width: 641px)` block in the `<style>` section (after the existing `@media (max-width: 640px)` block). Also update a few **default** styles:

- [ ] In default styles: hide `#last-updated`, hide `#desktop-wsel-wrap` (shown only via min-width block), hide `#desktop-hero`, hide `#desktop-right`, hide `#desktop-donut-card`, hide `#desktop-alarms-widget`, hide `#desktop-health-card`.

```css
#last-updated { display: none; }
#desktop-wsel-wrap { display: none; }
#desktop-hero { display: none; }
#desktop-right { display: none; }
.alarms-overlay { display: none; }
.alarms-flyout { display: none; }
```

- [ ] Add `@media (min-width: 641px)` block:

```css
@media (min-width: 641px) {
  /* Show desktop window selector in header; hide mobile one */
  #desktop-wsel-wrap   { display: inline-block; }
  #header-window-select { display: none; }

  /* Hide window selector inside chart card */
  #window-select { display: none; }

  /* Two-column main grid */
  main {
    max-width: 1280px;
    margin: 0 auto;
    padding: 1.25rem 1.5rem;
    display: grid;
    grid-template-columns: 1fr 300px;
    gap: 1rem;
    align-items: start;
  }

  /* Left column: stacks chart card, distribution strip, stat panels */
  #desktop-hero,
  .card:has(#bg-chart),
  #distribution-strip,
  #tab-stats {
    grid-column: 1;
  }

  /* Right column */
  #desktop-right {
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    grid-column: 2;
    grid-row: 1 / 99;
  }

  /* Show desktop hero */
  #desktop-hero {
    display: block;
    grid-row: 1;
  }

  /* Show distribution strip on desktop */
  #distribution-strip {
    display: block;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 0.75rem;
    box-shadow: var(--shadow);
    padding: 1rem 1.125rem;
  }
  #distribution-strip .dist-label { display: none; } /* hide "DISTRIBUTION · Xh" label; desktop adds its own */

  /* Show both tab panels; hide tab bar */
  .tab-bar    { display: none; }
  .tab-panel  { display: block; }

  /* Desktop hero card layout */
  #dh-body {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }
  #dh-left {
    display: flex;
    align-items: baseline;
    gap: 0.25rem;
    flex-shrink: 0;
  }
  #dh-val { font-size: 3.5rem; font-weight: 800; letter-spacing: -0.02em; }
  .dh-unit { font-size: 0.875rem; color: var(--text-muted); }
  .dh-divider {
    width: 1px; align-self: stretch;
    background: var(--border); margin: 0.25rem 0; flex-shrink: 0;
  }
  #dh-meta {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 0 2rem;
    flex: 1;
  }
  .dh-meta-label {
    font-size: 0.6875rem; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 0.2rem;
  }
  .dh-meta-val {
    font-size: 1.625rem; font-weight: 700; line-height: 1;
    font-variant-numeric: tabular-nums;
  }
  .dh-meta-age { font-size: 1.25rem; color: var(--text-muted); }

  /* Stats grid: 3-col row (Avg / StdDev / CV) on desktop */
  #tab-stats .stats-grid {
    grid-template-columns: 1fr 1fr 1fr;
  }
  /* Hide redundant cards that are now in hero or not needed on desktop */
  #tab-stats .stat-current,
  #tab-stats .stat-mobile-hide,
  #tab-stats .stat-tir,
  #tab-stats #stat-time-low,
  #tab-stats #stat-time-high { display: none; }
  /* Show the three stat cards we want */
  #tab-stats #stat-avg,
  #tab-stats #stat-stddev,
  #tab-stats #stat-cv { display: block; }

  /* Hide alarms tab panel on desktop (alarms live in the flyout) */
  #tab-alarms { display: none; }

  /* Distribution strip desktop label */
  #distribution-strip::before {
    content: 'DISTRIBUTION';
    display: block;
    font-size: 0.6875rem; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 0.75rem;
  }

  /* Chart y-axis label (applied via JS, no CSS needed) */

  /* Right panel widgets */
  #desktop-right .card { width: 100%; }

  /* Donut canvas */
  #desktop-donut-wrap {
    display: flex; justify-content: center; padding: 0.5rem 0 0.75rem;
  }
  #desktop-donut-canvas { width: 160px !important; height: 160px !important; }

  /* TIR legend */
  #desktop-tir-legend { display: flex; flex-direction: column; gap: 0.4rem; margin-top: 0.25rem; }
  .desktop-tir-row { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; }
  .desktop-tir-swatch { width: 10px; height: 10px; border-radius: 3px; flex-shrink: 0; }
  .desktop-tir-name { color: var(--text-muted); flex: 1; }
  .desktop-tir-pct { font-weight: 700; }

  /* Alarms widget */
  #desktop-alarms-widget { cursor: pointer; transition: border-color 0.15s; }
  #desktop-alarms-widget:hover { border-color: #475569; }
  .desktop-widget-header {
    display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.625rem;
  }
  .desktop-view-all { font-size: 0.75rem; color: var(--text-muted); }
  .desktop-alarm-row { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0; font-size: 0.825rem; }
  .desktop-alarm-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
  .desktop-alarm-name { flex: 1; }
  .desktop-alarm-divider { height: 1px; background: var(--border); margin: 0.2rem 0; }

  /* Health widget */
  .desktop-health-row { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0; font-size: 0.825rem; }
  .desktop-health-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; flex-shrink: 0; }
  .desktop-health-name { flex: 1; }
  .desktop-health-status { font-size: 0.75rem; font-weight: 600; }
  .desktop-health-sub { font-size: 0.7rem; color: var(--text-muted); padding-left: 1.375rem; margin-top: -0.1rem; padding-bottom: 0.2rem; }
  .desktop-health-divider { height: 1px; background: var(--border); margin: 0.2rem 0; }

  /* Flyout overlay */
  .alarms-overlay.open {
    display: block;
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.45); z-index: 200;
  }
  /* Flyout panel */
  .alarms-flyout.open {
    display: flex; flex-direction: column;
    position: fixed; top: 0; right: 0; bottom: 0; width: 480px;
    background: var(--surface); border-left: 1px solid var(--border);
    z-index: 201; box-shadow: -4px 0 24px rgba(0,0,0,0.35);
  }
  .alarms-flyout-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 1rem 1.25rem; border-bottom: 1px solid var(--border); flex-shrink: 0;
  }
  .alarms-flyout-title { font-size: 1rem; font-weight: 700; }
  .alarms-flyout-close {
    background: none; border: 1px solid var(--border); border-radius: 0.375rem;
    padding: 0.2rem 0.5rem; cursor: pointer; font-size: 1rem; color: var(--text-muted);
  }
  .alarms-flyout-body { flex: 1; overflow-y: auto; padding: 1.25rem; display: flex; flex-direction: column; gap: 1rem; }
  .alarms-flyout-section-title {
    font-size: 0.6875rem; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 0.625rem;
  }
  .alarms-table { width: 100%; border-collapse: collapse; font-size: 0.825rem; }
  .alarms-table th {
    text-align: left; padding: 0.35rem 0.625rem;
    font-size: 0.6875rem; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.06em; color: var(--text-muted); border-bottom: 1px solid var(--border);
  }
  .alarms-table td { padding: 0.55rem 0.625rem; border-bottom: 1px solid var(--border); }
  .alarms-table tr:last-child td { border-bottom: none; }
}
```

- [ ] Verify desktop layout renders with two columns; verify mobile is unchanged at ≤640px.

- [ ] Commit:
```bash
git add dashboard/static/index.html
git commit -m "feat(dashboard): add desktop CSS — two-column layout, hero, donut, flyout"
```

---

### Task 6: Add desktop JavaScript

**Files:**
- Modify: `dashboard/static/index.html` (script block)

- [ ] Add `toggleDesktopWindowSelect()` and close-on-outside-click handling for `#desktop-wsel-wrap`. In `updateWindowLabels(w)`, also update `#desktop-wsel-label`.

```js
function toggleDesktopWindowSelect() {
  document.getElementById('desktop-wsel-wrap').classList.toggle('open');
}
// in the existing document click listener, add:
const dws = document.getElementById('desktop-wsel-wrap');
if (dws && !dws.contains(e.target)) dws.classList.remove('open');
// in updateWindowLabels(w), add:
const dwLabel = document.getElementById('desktop-wsel-label');
if (dwLabel) dwLabel.textContent = w;
```

- [ ] Add `renderDesktopWidgets(d)` function called from the existing `renderWidgets(d)`:

```js
function renderDesktopWidgets(d) {
  if (window.matchMedia('(max-width: 640px)').matches) return;

  const cur = d.current;
  const prev = d.previous;

  // Hero card
  const dhVal = document.getElementById('dh-val');
  if (dhVal) {
    dhVal.textContent = cur ? cur.value : '—';
    dhVal.className = 'stat-value' + (cur ? ' ' + bgClass(cur.value) : '');
  }
  const dhDelta = document.getElementById('dh-delta');
  if (dhDelta && cur && prev) {
    const delta = cur.value - prev.value;
    dhDelta.textContent = (delta >= 0 ? '+' : '') + delta;
    dhDelta.className = 'dh-meta-val ' + bgClass(cur.value);
  }
  const dhTrend = document.getElementById('dh-trend');
  if (dhTrend) dhTrend.textContent = cur ? (TREND_ARROW[cur.trend] || '—') : '—';
  const dhAge = document.getElementById('dh-age');
  if (dhAge) dhAge.textContent = cur ? timeAgo(cur.recorded_at) : '—';

  // Donut chart
  renderDesktopDonut(d);

  // Alarms widget
  renderDesktopAlarms(d);

  // Health widget
  renderDesktopHealth(d);
}
```

- [ ] Add `renderDesktopDonut(d)`:

```js
let desktopDonut = null;
function renderDesktopDonut(d) {
  const canvas = document.getElementById('desktop-donut-canvas');
  if (!canvas) return;
  const stats = d.stats || {};
  const tbr = stats.time_below_range ?? 0;
  const tir = stats.time_in_range ?? 0;
  const tar = stats.time_above_range ?? 0;
  const data = [tbr, tir, tar];
  const colors = [cssVar('--low'), cssVar('--normal'), cssVar('--high')];

  if (desktopDonut) {
    desktopDonut.data.datasets[0].data = data;
    desktopDonut.update('none');
  } else {
    desktopDonut = new Chart(canvas.getContext('2d'), {
      type: 'doughnut',
      data: {
        datasets: [{
          data,
          backgroundColor: colors,
          borderWidth: 0,
          hoverOffset: 0,
        }],
      },
      options: {
        cutout: '68%',
        responsive: false,
        plugins: {
          legend: { display: false },
          tooltip: { enabled: false },
        },
        animation: false,
      },
      plugins: [{
        id: 'centerText',
        afterDraw(chart) {
          const { ctx, chartArea: { width, height, left, top } } = chart;
          ctx.save();
          ctx.textAlign = 'center';
          ctx.textBaseline = 'middle';
          const cx = left + width / 2, cy = top + height / 2;
          ctx.font = 'bold 20px system-ui, sans-serif';
          ctx.fillStyle = cssVar('--normal');
          ctx.fillText(tir + '%', cx, cy - 8);
          ctx.font = '9px system-ui, sans-serif';
          ctx.fillStyle = cssVar('--text-muted');
          ctx.fillText('in range', cx, cy + 10);
          ctx.restore();
        },
      }],
    });
  }

  // Legend
  const legend = document.getElementById('desktop-tir-legend');
  if (legend) {
    const rows = [
      { label: 'Below range', pct: tbr, color: cssVar('--low') },
      { label: 'In range',    pct: tir, color: cssVar('--normal') },
      { label: 'Above range', pct: tar, color: cssVar('--high') },
    ];
    legend.innerHTML = rows.map(r =>
      `<div class="desktop-tir-row">
        <span class="desktop-tir-swatch" style="background:${r.color}"></span>
        <span class="desktop-tir-name">${r.label}</span>
        <span class="desktop-tir-pct" style="color:${r.color}">${r.pct}%</span>
      </div>`
    ).join('');
  }
}
```

- [ ] Add `renderDesktopAlarms(d)`:

```js
function renderDesktopAlarms(d) {
  const container = document.getElementById('desktop-alarms-rows');
  if (!container) return;
  const alarms = (d.alarms || []).slice(0, 3);
  if (!alarms.length) { container.innerHTML = '<div style="color:var(--text-muted);font-size:0.8rem;">No alarms configured</div>'; return; }

  const statusColor = s => s === 'active' ? 'var(--low)' : 'var(--text-muted)';
  const badgeClass  = s => s === 'active' ? 'badge-active' : '';
  const badgeLabel  = s => ({ active: 'Active', snoozed_until: 'Snoozed', fired: 'OK', never_fired: 'OK' }[s] || s);

  container.innerHTML = alarms.map((a, i) =>
    `${i > 0 ? '<div class="desktop-alarm-divider"></div>' : ''}
     <div class="desktop-alarm-row">
       <div class="desktop-alarm-dot" style="background:${statusColor(a.status)}"></div>
       <span class="desktop-alarm-name" style="color:${a.status === 'active' ? 'var(--text)' : 'var(--text-muted)'}">${esc(a.name)}</span>
       <span class="badge ${badgeClass(a.status)}">${badgeLabel(a.status)}</span>
     </div>`
  ).join('');
}
```

- [ ] Add flyout open/close + population:

```js
function openAlarmsFlyout() {
  document.getElementById('alarms-overlay').classList.add('open');
  document.getElementById('alarms-flyout').classList.add('open');
  if (lastData) populateAlarmsFlyout(lastData);
}
function closeAlarmsFlyout() {
  document.getElementById('alarms-overlay').classList.remove('open');
  document.getElementById('alarms-flyout').classList.remove('open');
}
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeAlarmsFlyout(); });

function populateAlarmsFlyout(d) {
  const tbody = document.getElementById('flyout-alarm-rows');
  if (tbody) {
    const alarms = d.alarms || [];
    tbody.innerHTML = alarms.map(a => {
      const fired = a.last_fired_at ? timeAgo(a.last_fired_at) : '—';
      const statusLabel = { active: 'Active', snoozed_until: 'Snoozed', fired: 'OK', never_fired: '—' }[a.status] || a.status;
      const statusStyle = a.status === 'active' ? 'color:var(--low);font-weight:600;' : 'color:var(--text-muted);';
      return `<tr>
        <td>${esc(a.name)}</td>
        <td><span class="badge ${a.priority === 'emergency' ? 'badge-emergency' : a.priority === 'high' ? 'badge-high' : 'badge-normal'}">${esc(a.priority)}</span></td>
        <td style="color:var(--text-muted)">${fired}</td>
        <td style="${statusStyle}">${statusLabel}</td>
      </tr>`;
    }).join('');
  }
  const hbody = document.getElementById('flyout-history-rows');
  if (hbody) {
    const history = d.alarm_history || [];
    hbody.innerHTML = history.slice(0, 20).map(h =>
      `<tr>
        <td>${esc(h.alarm_name)}</td>
        <td style="color:${h.bg_value < (lastData?.target?.low ?? 70) ? 'var(--low)' : h.bg_value > (lastData?.target?.high ?? 180) ? 'var(--high)' : 'var(--normal)'}">${h.bg_value}</td>
        <td style="color:var(--text-muted)">${fmtTime(h.fired_at)}</td>
      </tr>`
    ).join('') || '<tr><td colspan="3" style="color:var(--text-muted)">No history</td></tr>';
  }
}
```

- [ ] Add `renderDesktopHealth(d)`:

```js
function renderDesktopHealth(d) {
  // Dexcom API: derived from current reading age
  const dexcomDot    = document.getElementById('dh-dexcom-dot');
  const dexcomStatus = document.getElementById('dh-dexcom-status');
  const dexcomSub    = document.getElementById('dh-dexcom-sub');
  if (dexcomDot && dexcomStatus) {
    const cur = d.current;
    if (!cur) {
      dexcomDot.style.background = 'var(--low)';
      dexcomStatus.textContent = 'Error';
      dexcomStatus.style.color = 'var(--low)';
      if (dexcomSub) dexcomSub.textContent = 'No readings';
    } else {
      const ageMs = Date.now() - new Date(cur.recorded_at).getTime();
      const ageMin = ageMs / 60000;
      if (ageMin < 10) {
        dexcomDot.style.background = 'var(--normal)';
        dexcomStatus.textContent = 'OK';
        dexcomStatus.style.color = 'var(--normal)';
      } else if (ageMin < 30) {
        dexcomDot.style.background = 'var(--high)';
        dexcomStatus.textContent = 'Delayed';
        dexcomStatus.style.color = 'var(--high)';
      } else {
        dexcomDot.style.background = 'var(--low)';
        dexcomStatus.textContent = 'Error';
        dexcomStatus.style.color = 'var(--low)';
      }
      if (dexcomSub) dexcomSub.textContent = 'Last reading ' + timeAgo(cur.recorded_at);
    }
  }

  // Watchdog
  const wdDot    = document.getElementById('dh-watchdog-dot');
  const wdStatus = document.getElementById('dh-watchdog-status');
  const wdSub    = document.getElementById('dh-watchdog-sub');
  const health   = d.health;
  if (wdDot && wdStatus) {
    if (!health?.watchdog?.configured) {
      wdDot.style.background = 'var(--text-muted)';
      wdStatus.textContent = '—';
      wdStatus.style.color = 'var(--text-muted)';
      if (wdSub) wdSub.textContent = 'Not configured';
    } else if (!health.watchdog.last_ping_at) {
      wdDot.style.background = 'var(--text-muted)';
      wdStatus.textContent = 'Pending';
      wdStatus.style.color = 'var(--text-muted)';
      if (wdSub) wdSub.textContent = 'No ping recorded';
    } else {
      const pingAgeMs = Date.now() - new Date(health.watchdog.last_ping_at).getTime();
      const pingMin = pingAgeMs / 60000;
      if (pingMin < 10) {
        wdDot.style.background = 'var(--normal)';
        wdStatus.textContent = 'OK';
        wdStatus.style.color = 'var(--normal)';
      } else if (pingMin < 30) {
        wdDot.style.background = 'var(--high)';
        wdStatus.textContent = 'Delayed';
        wdStatus.style.color = 'var(--high)';
      } else {
        wdDot.style.background = 'var(--low)';
        wdStatus.textContent = 'Error';
        wdStatus.style.color = 'var(--low)';
      }
      if (wdSub) wdSub.textContent = 'Last ping ' + timeAgo(health.watchdog.last_ping_at);
    }
  }
}
```

- [ ] In the chart options (inside `renderChart`), change the y-scale to position right:
```js
y: {
  position: 'right',   // add this line
  suggestedMin: 40,
  grid: { color: cssVar('--grid') },
  ticks: { color: cssVar('--text-muted') },
},
```

- [ ] Wire `renderDesktopWidgets(d)` into `renderWidgets(d)` — add call at the end of that function:
```js
renderDesktopWidgets(d);
```

- [ ] Also destroy `desktopDonut` in `toggleTheme()` (after `chart.destroy()`):
```js
if (desktopDonut) { desktopDonut.destroy(); desktopDonut = null; }
```

- [ ] Run: `go run . -config config.toml`, verify desktop renders correctly, verify mobile unchanged.

- [ ] Commit:
```bash
git add dashboard/static/index.html
git commit -m "feat(dashboard): add desktop JS — donut, health widget, alarms flyout"
```

---

### Task 7: Commit spec + push all branches

- [ ] Commit spec and plan:
```bash
git add docs/superpowers/specs/2026-05-29-desktop-dashboard-redesign.md
git add docs/superpowers/plans/2026-05-29-desktop-dashboard-redesign.md
git commit -m "docs: add desktop dashboard redesign spec and plan"
```

- [ ] Run full test suite: `go test ./...` — expect PASS

- [ ] Push this branch:
```bash
git push -u origin feature/desktop-dashboard-redesign
```

- [ ] Push stats branch:
```bash
git push origin feature/dashboard-upgrades
```
