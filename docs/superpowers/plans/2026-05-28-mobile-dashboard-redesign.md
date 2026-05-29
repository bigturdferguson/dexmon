# Mobile Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the mobile (≤640px) dashboard to fit all stats on one screen with Inter font, a window dropdown in the app header, current BG hero card above the chart, a quartile distribution strip, and full-width stat rows.

**Architecture:** All changes are confined to `dashboard/static/index.html`. Desktop layout is untouched — every addition is either inside `@media (max-width: 640px)` or paired with a `display:none` default that's overridden on mobile. New DOM elements are added at specific structural locations in `<head>`, `<header>`, and `<main>`. JS additions slot into existing function bodies.

**Tech Stack:** Vanilla HTML/CSS/JS, Chart.js 4.4.4, Inter font via Google Fonts CDN (mobile-only load via `media="(max-width: 640px)"`).

---

## File Map

| File | What changes |
|------|-------------|
| `dashboard/static/index.html` | All changes — HTML, CSS, JS |

---

## Task 1: Inter font

**Files:**
- Modify: `dashboard/static/index.html` — `<head>` + existing mobile CSS block

- [ ] **Step 1: Add font preconnect and stylesheet link**

Find this line in `<head>` (line ~5):
```html
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
```

Add immediately after it:
```html
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap"
        media="(max-width: 640px)" rel="stylesheet">
```

- [ ] **Step 2: Apply Inter in mobile CSS**

Find the opening of the existing mobile media query (line ~80):
```css
    @media (max-width: 640px) {
      .stats-grid { grid-template-columns: 1fr 1fr; }
```

Add `body { font-family: ... }` as the first rule inside it:
```css
    @media (max-width: 640px) {
      body { font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif; }
      .stats-grid { grid-template-columns: 1fr 1fr; }
```

- [ ] **Step 3: Verify visually**

Start the server (requires config — skip automated test, verify manually):
```
go run . -config config.toml
```
Open `http://localhost:8080` in Chrome DevTools with viewport set to 375px. Open the Network tab and confirm `fonts.googleapis.com` is fetched. On desktop (>640px) confirm no font request.

- [ ] **Step 4: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): load Inter font on mobile via media-conditional link"
```

---

## Task 2: App header — window-select dropdown

**Files:**
- Modify: `dashboard/static/index.html` — `<header>` HTML, CSS rules, JS functions

- [ ] **Step 1: Add `#header-window-select` HTML inside `.header-right`**

Find this in the `<header>` block:
```html
    <div class="header-right">
      <span id="last-updated">Loading…</span>
      <button id="theme-btn" onclick="toggleTheme()">☀︎</button>
    </div>
```

Replace with:
```html
    <div class="header-right">
      <span id="last-updated">Loading…</span>
      <div class="wsel-wrap" id="header-window-select">
        <button class="wsel-btn" id="header-wsel-btn" onclick="toggleHeaderWindowSelect()">
          <span id="header-wsel-label">24h</span>
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
      <button id="theme-btn" onclick="toggleTheme()">☀︎</button>
    </div>
```

- [ ] **Step 2: Add desktop-default hide rule for `#header-window-select`**

Find this CSS block (just before the mobile media query, around line 60):
```css
    #last-updated { font-size: 0.8rem; color: var(--text-muted); }
```

Add immediately after it:
```css
    #header-window-select { display: none; }
```

- [ ] **Step 3: Add mobile show/hide rules**

Find the opening of the `@media (max-width: 640px)` block. After the `body { font-family: ... }` line added in Task 1, add:
```css
      #last-updated { display: none; }
      #header-window-select { display: inline-block; }
      #window-select { display: none; }
```

Full context around the insertion:
```css
    @media (max-width: 640px) {
      body { font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif; }
      #last-updated { display: none; }
      #header-window-select { display: inline-block; }
      #window-select { display: none; }
      .stats-grid { grid-template-columns: 1fr 1fr; }
```

- [ ] **Step 4: Add `toggleHeaderWindowSelect` JS function**

Find:
```js
    function toggleWindowSelect() {
      document.getElementById('window-select').classList.toggle('open');
    }
```

Add immediately after it:
```js
    function toggleHeaderWindowSelect() {
      document.getElementById('header-window-select').classList.toggle('open');
    }
```

- [ ] **Step 5: Update `updateWindowLabels` to sync header dropdown**

Find:
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

Replace with:
```js
    function updateWindowLabels(w) {
      ['high', 'low', 'avg', 'stddev', 'cv', 'tir', 'quartiles'].forEach(id => {
        const el = document.getElementById('sub-' + id);
        if (el) el.textContent = w;
      });
      document.getElementById('chart-title').textContent = chartTitle(w);
      document.getElementById('wsel-label').textContent = w;
      const hLabel = document.getElementById('header-wsel-label');
      if (hLabel) hLabel.textContent = w;
      document.querySelectorAll('.wsel-item').forEach(b => {
        b.classList.toggle('active', b.dataset.window === w);
      });
    }
```

- [ ] **Step 6: Update `setWindow` to close the header dropdown**

Find:
```js
    function setWindow(w) {
      document.getElementById('window-select').classList.remove('open');
      currentWindow = w;
```

Replace with:
```js
    function setWindow(w) {
      document.getElementById('window-select').classList.remove('open');
      const hws = document.getElementById('header-window-select');
      if (hws) hws.classList.remove('open');
      currentWindow = w;
```

- [ ] **Step 7: Update click-outside listener to also close header dropdown**

Find:
```js
    document.addEventListener('click', function(e) {
      const ws = document.getElementById('window-select');
      if (ws && !ws.contains(e.target)) ws.classList.remove('open');
    });
```

Replace with:
```js
    document.addEventListener('click', function(e) {
      const ws  = document.getElementById('window-select');
      const hws = document.getElementById('header-window-select');
      if (ws  && !ws.contains(e.target))  ws.classList.remove('open');
      if (hws && !hws.contains(e.target)) hws.classList.remove('open');
    });
```

- [ ] **Step 8: Verify visually**

At 375px viewport: app header shows **dexmon · [24h ▼] · ☀︎** (no "Loading…"). Tapping "24h ▼" opens the dropdown. Selecting a window closes it and refreshes data. At >640px: header shows only **dexmon · Loading… · ☀︎** (no dropdown).

- [ ] **Step 9: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): add window-select dropdown in app header; hide on desktop"
```

---

## Task 3: Chart card — "READINGS" label on mobile

**Files:**
- Modify: `dashboard/static/index.html` — chart card HTML + CSS

- [ ] **Step 1: Add `.chart-title-mobile` span and update chart card header**

Find:
```html
      <div class="card-header card-header-row">
        <div class="card-title" id="chart-title">BG</div>
```

Replace with:
```html
      <div class="card-header card-header-row">
        <div>
          <div class="card-title" id="chart-title">BG</div>
          <span class="card-title chart-title-mobile">READINGS</span>
        </div>
```

- [ ] **Step 2: Add desktop-default hide for `.chart-title-mobile`**

Find:
```css
    #header-window-select { display: none; }
```

Add immediately after it:
```css
    .chart-title-mobile { display: none; }
```

- [ ] **Step 3: Add mobile show/hide rules for chart title**

Inside the `@media (max-width: 640px)` block, after the `#window-select { display: none; }` line added in Task 2 Step 3:
```css
      #chart-title { display: none; }
      .chart-title-mobile { display: block; }
```

Full context:
```css
      #last-updated { display: none; }
      #header-window-select { display: inline-block; }
      #window-select { display: none; }
      #chart-title { display: none; }
      .chart-title-mobile { display: block; }
      .stats-grid { grid-template-columns: 1fr 1fr; }
```

- [ ] **Step 4: Verify visually**

At 375px: chart card shows "READINGS" (not "BG" or "24h"). At >640px: chart card shows "BG".

- [ ] **Step 5: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): show 'READINGS' label in chart card header on mobile"
```

---

## Task 4: Mobile hero — Current BG card

**Files:**
- Modify: `dashboard/static/index.html` — `<main>` HTML, CSS, `renderWidgets` JS

- [ ] **Step 1: Add `#mobile-hero` as first child of `<main>`**

Find:
```html
  <main>
    <div class="card">
      <div class="card-header card-header-row">
```

Replace with:
```html
  <main>
    <div id="mobile-hero">
      <div class="mobile-hero-cols">
        <div class="mobile-hero-col">
          <div class="mobile-hero-val" id="mh-current">—</div>
          <div class="mobile-hero-sub">mg/dL · <span id="mh-age">—</span></div>
        </div>
        <div class="mobile-hero-div"></div>
        <div class="mobile-hero-col">
          <div class="mobile-hero-mid" id="mh-delta">—</div>
          <div class="mobile-hero-sub">change</div>
        </div>
        <div class="mobile-hero-div"></div>
        <div class="mobile-hero-col">
          <div class="mobile-hero-mid" id="mh-trend">—</div>
          <div class="mobile-hero-sub">trend</div>
        </div>
      </div>
    </div>
    <div class="card">
      <div class="card-header card-header-row">
```

- [ ] **Step 2: Add `stat-mobile-hide` to `.stat-current` card**

Find:
```html
        <div class="stat-card stat-current">
```

Replace with:
```html
        <div class="stat-card stat-current stat-mobile-hide">
```

- [ ] **Step 3: Add desktop-default hide for `#mobile-hero`**

Find:
```css
    #header-window-select { display: none; }
    .chart-title-mobile { display: none; }
```

Add immediately after:
```css
    #mobile-hero { display: none; }
```

- [ ] **Step 4: Add mobile CSS for hero card**

Inside the `@media (max-width: 640px)` block, after the `.chart-title-mobile { display: block; }` line added in Task 3:
```css
      #mobile-hero {
        display: block;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 0.75rem;
        box-shadow: var(--shadow);
        margin-bottom: 0.75rem;
      }
      .mobile-hero-cols {
        display: flex; align-items: stretch; padding: 0.5rem;
      }
      .mobile-hero-col {
        flex: 1; display: flex; flex-direction: column;
        align-items: center; justify-content: center; gap: 0.15rem;
      }
      .mobile-hero-val {
        font-size: 2rem; font-weight: 700; line-height: 1;
        font-variant-numeric: tabular-nums;
      }
      .mobile-hero-mid {
        font-size: 1.1rem; font-weight: 700; line-height: 1;
        font-variant-numeric: tabular-nums;
      }
      .mobile-hero-sub {
        font-size: 0.65rem; color: var(--text-muted); text-align: center;
      }
      .mobile-hero-div {
        width: 1px; background: var(--border);
        margin: 0.15rem 0.1rem; align-self: stretch;
      }
```

- [ ] **Step 5: Update `renderWidgets` to populate mobile hero**

Find the end of the `renderWidgets` function — specifically the line where `deltaEl.textContent` is set and just before the `prevEl` section:

```js
      const deltaEl = document.getElementById('val-delta');
      if (deltaEl) {
        if (cur && prev) {
          const delta = cur.value - prev.value;
          deltaEl.textContent = (delta >= 0 ? '+' : '') + delta;
        } else {
          deltaEl.textContent = '';
        }
      }

      const prevEl = document.getElementById('val-prev');
```

Replace with:
```js
      const deltaEl = document.getElementById('val-delta');
      let deltaText = '';
      if (deltaEl) {
        if (cur && prev) {
          const delta = cur.value - prev.value;
          deltaText = (delta >= 0 ? '+' : '') + delta;
          deltaEl.textContent = deltaText;
        } else {
          deltaEl.textContent = '';
        }
      }

      // Mobile hero
      const mhCur = document.getElementById('mh-current');
      if (mhCur) {
        mhCur.textContent = cur ? cur.value : '—';
        mhCur.className = 'mobile-hero-val' + (cur ? ' ' + bgClass(cur.value) : '');
      }
      const mhAge = document.getElementById('mh-age');
      if (mhAge) mhAge.textContent = cur ? timeAgo(cur.recorded_at) : '—';
      const mhDelta = document.getElementById('mh-delta');
      if (mhDelta) mhDelta.textContent = deltaText || '—';
      const mhTrend = document.getElementById('mh-trend');
      if (mhTrend) mhTrend.textContent = cur ? (TREND_ARROW[cur.trend] || '—') : '—';

      const prevEl = document.getElementById('val-prev');
```

- [ ] **Step 6: Verify visually**

At 375px: a new card appears at the top of the page above the chart, showing `180` (colored amber/green/red), `mg/dL · 3m ago`, `+2` (change), `→` (trend) in three equal columns. The existing "Current BG" stat card is hidden. At >640px: no mobile-hero card visible; "Current BG" stat card is visible as before.

- [ ] **Step 7: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): add mobile hero card for current BG above chart"
```

---

## Task 5: Distribution strip widget

**Files:**
- Modify: `dashboard/static/index.html` — `<main>` HTML after chart card, CSS, new JS function, `fetchAndRender` call

- [ ] **Step 1: Add `#distribution-strip` HTML after chart card**

Find:
```html
    </div>

    <div class="tab-bar">
```

(This is the closing `</div>` of the chart `.card` and the start of the tab bar.) Replace with:
```html
    </div>

    <div id="distribution-strip">
      <div class="dist-label">Distribution</div>
      <div class="dist-strip-wrap" id="dist-strip-inner"></div>
      <div class="dist-axis">
        <span>40</span><span>70</span><span>130</span><span>180</span><span>260</span>
      </div>
    </div>

    <div class="tab-bar">
```

- [ ] **Step 2: Add desktop-default hide for `#distribution-strip`**

Find:
```css
    #mobile-hero { display: none; }
```

Add immediately after:
```css
    #distribution-strip { display: none; }
```

- [ ] **Step 3: Add mobile CSS for distribution strip**

Inside the `@media (max-width: 640px)` block, after the `.mobile-hero-div { ... }` rule added in Task 4:
```css
      #distribution-strip {
        display: block;
        background: var(--surface); border: 1px solid var(--border);
        border-radius: 0.75rem; box-shadow: var(--shadow);
        padding: 0.6rem 0.75rem 0.5rem;
        margin-bottom: 0.75rem;
      }
      .dist-label {
        font-size: 0.6875rem; font-weight: 700; text-transform: uppercase;
        letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 0.5rem;
      }
      .dist-strip-wrap { position: relative; height: 42px; }
      .dist-axis {
        display: flex; justify-content: space-between; margin-top: 2px;
      }
      .dist-axis span { font-size: 0.6875rem; color: var(--text-muted); }
```

- [ ] **Step 4: Add `renderDistributionStrip` JS function**

Find:
```js
    function lttb(data, threshold) {
```

Add immediately before it:
```js
    const BG_STRIP_MIN = 40, BG_STRIP_MAX = 260;

    function renderDistributionStrip(d) {
      const wrap = document.getElementById('dist-strip-inner');
      if (!wrap) return;
      wrap.innerHTML = '';

      const q1  = d.stats?.q1,  med = d.stats?.median, q3 = d.stats?.q3;
      const low = d.target?.low, high = d.target?.high;
      if (!q1 || !med || !q3) return;

      const range = BG_STRIP_MAX - BG_STRIP_MIN;
      const pct  = v => ((v - BG_STRIP_MIN) / range * 100).toFixed(2) + '%';
      const wPct = (a, b) => ((b - a) / range * 100).toFixed(2) + '%';

      // Target zone background
      if (low && high) {
        const tz = document.createElement('div');
        tz.style.cssText = `position:absolute;left:${pct(low)};width:${wPct(low,high)};` +
          `top:14px;height:14px;background:rgba(22,163,74,0.1);` +
          `border:1px solid rgba(22,163,74,0.25);border-radius:3px;`;
        wrap.appendChild(tz);
      }

      // Track
      const track = document.createElement('div');
      track.style.cssText = 'position:absolute;left:0;right:0;top:19px;height:4px;' +
        'background:rgba(0,0,0,0.2);border-radius:2px;';
      wrap.appendChild(track);

      // IQR gradient fill
      const fill = document.createElement('div');
      fill.style.cssText = `position:absolute;left:${pct(q1)};width:${wPct(q1,q3)};` +
        `top:19px;height:4px;border-radius:2px;` +
        `background:linear-gradient(to right,rgba(99,102,241,0.5),rgba(99,102,241,0.9));`;
      wrap.appendChild(fill);

      // Median line
      const medEl = document.createElement('div');
      medEl.style.cssText = `position:absolute;left:${pct(med)};top:13px;` +
        `width:2px;height:16px;background:var(--text);border-radius:1px;transform:translateX(-1px);`;
      wrap.appendChild(medEl);

      // Q labels: value above track, name below track
      [[q1,'Q1','var(--text-muted)'],[med,'Med','var(--text)'],[q3,'Q3','var(--text-muted)']].forEach(([v,lbl,col])=>{
        const vEl = document.createElement('div');
        vEl.style.cssText = `position:absolute;left:${pct(v)};top:0;` +
          `transform:translateX(-50%);font-size:0.6875rem;font-weight:700;color:${col};white-space:nowrap;`;
        vEl.textContent = v;
        wrap.appendChild(vEl);
        const lEl = document.createElement('div');
        lEl.style.cssText = `position:absolute;left:${pct(v)};top:30px;` +
          `transform:translateX(-50%);font-size:0.6rem;color:var(--text-muted);white-space:nowrap;`;
        lEl.textContent = lbl;
        wrap.appendChild(lEl);
      });
    }

```

- [ ] **Step 5: Call `renderDistributionStrip` from `fetchAndRender`**

Find:
```js
        renderWidgets(d);
        renderChart(d.readings || []);
        renderAlarms(d.alarms || []);
```

Replace with:
```js
        renderWidgets(d);
        renderDistributionStrip(d);
        renderChart(d.readings || []);
        renderAlarms(d.alarms || []);
```

- [ ] **Step 6: Verify visually**

At 375px: a "DISTRIBUTION" card appears between the chart and the tab bar, showing a horizontal strip with a green target zone overlay, an indigo IQR gradient bar, a white median tick, and Q1/Med/Q3 labels with values above the track (e.g., `88`, `139`, `172`) and names below (`Q1`, `Med`, `Q3`). Axis tick marks `40 70 130 180 260` appear below the strip. At >640px: no distribution strip visible.

- [ ] **Step 7: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): add quartile distribution strip widget between chart and tabs"
```

---

## Task 6: Stat cards — full-width flex rows on mobile

**Files:**
- Modify: `dashboard/static/index.html` — CSS (replace existing mobile stat-card grid rules) + one HTML class addition

- [ ] **Step 1: Add `stat-mobile-hide` to the Quartiles stat card**

The Quartiles card must be hidden on mobile (the distribution strip replaces it). Find:
```html
        <div class="stat-card stat-full">
          <div class="stat-label">Quartiles <span id="sub-quartiles" style="font-weight:400;">24h</span></div>
```

Replace with:
```html
        <div class="stat-card stat-full stat-mobile-hide">
          <div class="stat-label">Quartiles <span id="sub-quartiles" style="font-weight:400;">24h</span></div>
```

- [ ] **Step 2: Change stats-grid to single-column on mobile**

Find in the `@media (max-width: 640px)` block:
```css
      .stats-grid { grid-template-columns: 1fr 1fr; }
```

Replace with:
```css
      .stats-grid { grid-template-columns: 1fr; }
```

- [ ] **Step 3: Replace the stat-card grid layout with flex rows**

Find the block that starts:
```css
      /* Avg, Std Dev, CV: label + period on same line, value below */
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) {
        display: grid;
        grid-template-columns: auto 1fr;
        grid-template-rows: auto auto;
        grid-template-areas:
          "label sub"
          "value value";
        column-gap: 0.3em;
      }
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) .stat-label {
        grid-area: label;
        margin-bottom: 0;
        align-self: baseline;
      }
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) .stat-sub {
        grid-area: sub;
        margin-top: 0;
        display: block;
        align-self: baseline;
      }
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) .stat-value {
        grid-area: value;
        margin-top: 0.35rem;
      }
      /* In Range: same label+period inline treatment */
      .stat-tir {
        display: grid;
        grid-template-columns: auto 1fr;
        grid-template-rows: auto auto;
        grid-template-areas:
          "label sub"
          "value value";
        column-gap: 0.3em;
      }
      .stat-tir .stat-label {
        grid-area: label;
        margin-bottom: 0;
        align-self: baseline;
      }
      .stat-tir .stat-sub {
        grid-area: sub;
        margin-top: 0;
        display: block;
        align-self: baseline;
      }
      .stat-tir .tir-val-row {
        grid-area: value;
        margin-top: 0.35rem;
      }
      .stat-full .quartile-cell .stat-sub {
        justify-content: center;
      }
```

Replace the entire block with:
```css
      /* Avg, Std Dev, CV: full-width flex row — label left, value right */
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.5rem 0.875rem;
      }
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) .stat-label {
        font-size: 0.6875rem;
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.06em;
        color: var(--text-muted);
        margin-bottom: 0;
      }
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) .stat-sub {
        display: none;
      }
      .stat-card:not(.stat-current):not(.stat-full):not(.stat-mobile-hide) .stat-value {
        font-size: 1.1rem;
        font-weight: 700;
      }
      /* In Range: full-width flex row — label left, value right, no bar */
      .stat-tir {
        display: flex; align-items: center; justify-content: space-between;
        padding: 0.5rem 0.875rem;
      }
      .stat-tir .stat-label {
        margin-bottom: 0; font-size: 0.6875rem; font-weight: 700;
        text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-muted);
      }
      .stat-tir .tir-val-row { font-size: 1.1rem; font-weight: 700; }
      .stat-tir .tir-bar-wrap { display: none; }
      .stat-tir .stat-sub { display: none; }
```

- [ ] **Step 4: Verify visually**

At 375px: Avg, Std Dev, CV, and In Range each render as full-width cards with their label (e.g., "AVG") on the left and numeric value (e.g., `139`) on the right in larger text. The Quartiles card is hidden. At >640px: all cards render in their original multi-column grid layout with no changes.

- [ ] **Step 5: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): full-width flex stat rows for Avg/StdDev/CV/InRange; hide Quartiles"
```

---

## Acceptance Criteria Check

Before marking complete, verify each item from the spec at 375px mobile viewport:

- [ ] Inter font loaded (check Network tab — fonts.googleapis.com request present)
- [ ] App header: **dexmon · [24h ▼] · ☀︎** (no "Last updated")
- [ ] Selecting window in header dropdown refreshes all widgets
- [ ] Mobile hero card at top: BG value colored, "mg/dL · Xm ago", change, trend — three equal columns
- [ ] Chart shows "READINGS" (not window string)
- [ ] Distribution strip between chart and tab bar with Q1/Med/Q3 labels on a 40–260 scale
- [ ] Avg / Std Dev / CV / In Range each full-width with label left, value right, ~1.1rem value
- [ ] Everything visible without scrolling on a 390×844 viewport (iPhone 14 Pro)
- [ ] Desktop layout unchanged at >640px
