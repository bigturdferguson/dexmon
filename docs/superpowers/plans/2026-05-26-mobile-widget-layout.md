# Mobile Widget Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle mobile stat widgets (≤640px) so Current BG columns stack value-over-label with consistent alignment, single-value cards show the period inline with the card title, and Quartiles labels are properly centered.

**Architecture:** All changes are CSS additions inside the existing `@media (max-width: 640px)` block in `dashboard/static/index.html`, plus two small HTML structural changes: (1) `display: contents` technique on Current BG cells via CSS to achieve a two-row grid, and (2) a class added to In Range's inner wrapper div to enable CSS targeting. No JavaScript changes.

**Tech Stack:** Vanilla HTML/CSS inside a single-file dashboard. No build step — edit the file, refresh the browser to verify.

---

### Reference: Current mobile CSS block (lines 80–86 as of this plan)

```css
@media (max-width: 640px) {
  .stats-grid { grid-template-columns: 1fr 1fr; }
  .stat-current { grid-column: 1 / -1; }
  .stat-mobile-hide { display: none; }
  .stats-grid .stat-tir { grid-column: auto; }
  .stat-tir .tir-bar-wrap { display: none; }
}
```

All new CSS goes inside this same `@media` block. Append to it — do not duplicate it.

---

### Task 1: Hide time-ago from Current BG header on mobile

**Files:**
- Modify: `dashboard/static/index.html` (inside the `@media (max-width: 640px)` block)

- [ ] **Step 1: Add CSS to hide the age element**

Inside the `@media (max-width: 640px)` block, append:

```css
.stat-current .stat-label-age { display: none; }
```

- [ ] **Step 2: Verify visually**

Open the dashboard in a browser, resize to ≤640px (or use DevTools mobile emulation). The Current BG header should show only "Current BG" with no time text on the right.

- [ ] **Step 3: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): hide time-ago from Current BG header"
```

---

### Task 2: Current BG — two-row grid for three columns

**Context:** The three columns currently use `display: flex; align-items: baseline` on each `.stat-current .quartile-cell`, rendering value and label side-by-side. We want value centered above, label centered below, with all three labels locked to the same vertical row.

**Approach:** Set `display: contents` on `.stat-current .quartile-cell` (mobile only) so each cell's children become direct grid items of the parent `.quartile-row` grid. Explicitly assign `grid-column` and `grid-row` to every child so placement is deterministic.

**Files:**
- Modify: `dashboard/static/index.html` (inside `@media (max-width: 640px)` block)

- [ ] **Step 1: Add grid layout for the quartile-row**

Inside the `@media (max-width: 640px)` block, append:

```css
/* Current BG: two-row grid — values row 1, labels row 2 */
.stat-current .quartile-row {
  display: grid;
  grid-template-columns: 1fr auto 1fr auto 1fr;
  grid-template-rows: auto auto;
  margin-top: 0.25rem;
}
.stat-current .quartile-cell {
  display: contents;
}
/* Column 1 — BG value + label */
.stat-current .quartile-cell:nth-child(1) .stat-value {
  grid-column: 1; grid-row: 1;
  justify-self: center;
  align-self: center;
}
.stat-current .quartile-cell:nth-child(1) .stat-sub-inline {
  grid-column: 1; grid-row: 2;
  text-align: center;
}
/* Column 2 — first divider */
.stat-current .quartile-divider:nth-child(2) {
  grid-column: 2; grid-row: 1 / 3;
  align-self: center;
  padding: 0 0.25rem;
}
/* Column 3 — delta value + label */
.stat-current .quartile-cell:nth-child(3) .stat-value {
  grid-column: 3; grid-row: 1;
  justify-self: center;
  align-self: center;
}
.stat-current .quartile-cell:nth-child(3) .stat-sub-inline {
  grid-column: 3; grid-row: 2;
  text-align: center;
}
/* Column 4 — second divider */
.stat-current .quartile-divider:nth-child(4) {
  grid-column: 4; grid-row: 1 / 3;
  align-self: center;
  padding: 0 0.25rem;
}
/* Column 5 — trend value + label */
.stat-current .quartile-cell:nth-child(5) .stat-value {
  grid-column: 5; grid-row: 1;
  justify-self: center;
  align-self: center;
}
.stat-current .quartile-cell:nth-child(5) .stat-sub-inline {
  grid-column: 5; grid-row: 2;
  text-align: center;
}
```

- [ ] **Step 2: Verify visually**

At ≤640px, the Current BG widget should show three columns with:
- Each value centered horizontally in its column
- All three labels ("mg/dL", "change", "trend") locked to the same vertical row
- Dividers centered vertically across both rows

- [ ] **Step 3: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): two-row grid layout for Current BG three columns"
```

---

### Task 3: Current BG — resize delta and trend on mobile

**Context:** The existing rules set `.quartile-cell .stat-value { font-size: 1.375rem }` and `.stat-current .quartile-cell:first-child .stat-value { font-size: 2.25rem }`. On mobile, delta needs 1.75rem and trend needs 2.75rem. Target by ID since these are unique elements.

**Files:**
- Modify: `dashboard/static/index.html` (inside `@media (max-width: 640px)` block)

- [ ] **Step 1: Add font-size overrides**

Inside the `@media (max-width: 640px)` block, append:

```css
#val-delta { font-size: 1.75rem; }
#val-trend { font-size: 2.75rem; }
```

- [ ] **Step 2: Verify**

At ≤640px:
- The BG number (e.g. "112") renders at 2.25rem (unchanged, largest)
- The delta (e.g. "+4") renders at 1.75rem (medium)
- The trend arrow (e.g. "↑↑") renders at 2.75rem (tallest)
- A double arrow like "↑↑" fits without overflowing its column

- [ ] **Step 3: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): resize delta to 1.75rem and trend to 2.75rem in Current BG"
```

---

### Task 4: Single-value widgets — period inline with card title

**Context:** Avg, Std Dev, CV currently render as `stat-label` (top) → `stat-value` → `stat-sub "24h"` (bottom). On mobile we want `stat-label + stat-sub` on the same line at top-left, with the value below them left-aligned.

In Range (`stat-tir`) has the same goal but its `stat-value` is wrapped in an anonymous `div` alongside the (hidden) bar. A class `tir-val-row` is added to that wrapper so CSS can target it cleanly.

**Files:**
- Modify: `dashboard/static/index.html`
  - HTML: add class `tir-val-row` to In Range's inner wrapper div (~line 348)
  - CSS: add grid rules inside `@media (max-width: 640px)` block

- [ ] **Step 1: Add class to In Range inner wrapper**

Find this HTML (around line 347–350):

```html
<div style="display:flex;align-items:center;gap:0.75rem;">
  <div class="stat-value" id="val-tir">—</div>
  <div class="tir-bar-wrap"><div class="tir-bar" id="tir-bar"></div></div>
</div>
```

Change to:

```html
<div class="tir-val-row" style="display:flex;align-items:center;gap:0.75rem;">
  <div class="stat-value" id="val-tir">—</div>
  <div class="tir-bar-wrap"><div class="tir-bar" id="tir-bar"></div></div>
</div>
```

- [ ] **Step 2: Add CSS grid for Avg / Std Dev / CV cards**

Inside the `@media (max-width: 640px)` block, append:

```css
/* Avg, Std Dev, CV: label + period on same line, value below */
.stat-card:not(.stat-current):not(.stat-full) {
  display: grid;
  grid-template-columns: auto 1fr;
  grid-template-rows: auto auto;
  grid-template-areas:
    "label sub"
    "value value";
  column-gap: 0.3em;
}
.stat-card:not(.stat-current):not(.stat-full) .stat-label {
  grid-area: label;
  margin-bottom: 0;
  align-self: baseline;
}
.stat-card:not(.stat-current):not(.stat-full) .stat-sub {
  grid-area: sub;
  margin-top: 0;
  display: block;
  align-self: baseline;
}
.stat-card:not(.stat-current):not(.stat-full) .stat-value {
  grid-area: value;
  margin-top: 0.35rem;
}
```

- [ ] **Step 3: Add CSS grid for In Range card**

Inside the `@media (max-width: 640px)` block, append:

```css
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
```

- [ ] **Step 4: Verify**

At ≤640px, Avg, Std Dev, CV, and In Range cards should each show:
- Top line: card title and "24h" (or current window) side-by-side at top-left, same baseline
- Value: left-justified below, at 1.75rem

- [ ] **Step 5: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): period label inline with card title for single-value widgets"
```

---

### Task 5: Quartiles — center Q1/Median/Q3 labels

**Context:** The Quartiles widget uses `.stat-sub` (which has `display: flex`) for the Q1/Median/Q3 labels. `text-align: center` on the parent `.quartile-cell` does not center flex content — `justify-content: center` is needed.

**Files:**
- Modify: `dashboard/static/index.html` (inside `@media (max-width: 640px)` block)

- [ ] **Step 1: Add justify-content rule**

Inside the `@media (max-width: 640px)` block, append:

```css
.stat-full .quartile-cell .stat-sub {
  justify-content: center;
}
```

- [ ] **Step 2: Verify**

At ≤640px, the Quartiles widget should show Q1, Median, Q3 labels centered under their respective values.

- [ ] **Step 3: Verify no desktop regression**

Resize to ≥641px and confirm:
- Current BG header shows the time-ago again
- Current BG three columns render inline (value + label side-by-side, baseline aligned)
- Avg/Std Dev/CV/In Range have stat-label on top and stat-sub below the value
- Quartiles layout unchanged from before

- [ ] **Step 4: Commit**

```bash
git add dashboard/static/index.html
git commit -m "feat(mobile): center Q1/Median/Q3 labels in Quartiles widget"
```

---

## Acceptance Checklist

After all tasks complete, verify at ≤640px:

- [ ] Current BG header: "Current BG" only, no time-ago
- [ ] Current BG three columns: value centered above, label centered below, all three labels at same vertical position
- [ ] Delta value: 1.75rem
- [ ] Trend arrow: 2.75rem; "↑↑" fits without overflow
- [ ] Avg, Std Dev, CV, In Range: card title + "24h" on same line at top-left
- [ ] Value in those cards: left-justified below label row
- [ ] Quartiles Q1/Median/Q3 labels: centered under values
- [ ] No regressions at ≥641px desktop
