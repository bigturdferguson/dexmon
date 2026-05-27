# Mobile Widget Layout — Design Spec

**Date:** 2026-05-26
**Scope:** `@media (max-width: 640px)` only — no desktop changes.
**File:** `dashboard/static/index.html`

---

## Summary

Three visual changes to the mobile stat widget grid:

1. **Current BG widget** — remove time-ago from header; stack each of the three columns (value on top, label below) with consistent vertical anchoring; resize delta and trend values.
2. **Single-value widgets** (Avg, Std Dev, CV, In Range) — move the period label ("24h") inline next to the card title; left-justify the value.
3. **Quartiles widget** — stack each column (value on top, label below) centered, matching the Current BG treatment.

---

## 1. Current BG Widget

### 1a. Remove time-ago from header

The header currently renders `Current BG` on the left and the age (`val-current-age`, e.g. "5m ago") on the right via flex `space-between`. On mobile:

- Hide the `#val-current-age` element (CSS `display: none`).
- The `.stat-current .stat-label` flex row can remain; it simply collapses to left-aligned with one child.

### 1b. Three-column layout — stacked and anchored

Currently each `.stat-current .quartile-cell` renders value + label inline (flex row, baseline-aligned). On mobile:

- Each cell becomes a **flex column** (`flex-direction: column; align-items: center`).
- The `.quartile-row` uses a **CSS grid** with `grid-template-columns: 1fr auto 1fr auto 1fr` and `grid-template-rows: auto auto` so that all three value cells share row 1 and all three label cells share row 2 — locking labels to the same vertical position regardless of value font-size differences.
- Dividers (`.quartile-divider`) span both rows (`grid-row: 1 / 3`) and self-center.

To achieve the two-row grid with the current HTML (where value and label are siblings inside `.quartile-cell`), the implementation should either:
- (Preferred) Set `display: contents` on `.stat-current .quartile-cell` on mobile so its children become direct grid items, OR
- Restructure the HTML to have a separate value row and label row.

### 1c. Font sizes (mobile overrides only)

| Column | Element | Mobile font-size |
|--------|---------|-----------------|
| BG value | `#val-current` | `2.25rem` (unchanged) |
| Delta | `#val-delta` | `1.75rem` (was `1.375rem` inline) |
| Trend arrow | `#val-trend` | `2.75rem` (was `1.375rem` inline) |

The trend size was validated against the widest arrow strings (`↑↑`, `↓↓`) in the 1fr column — they fit.

---

## 2. Single-Value Widgets (Avg, Std Dev, CV, In Range)

Visible on mobile: Avg, Std Dev, CV (all plain `.stat-card`), In Range (`.stat-card.stat-full.stat-tir`).

### 2a. Period label inline with card title

Currently: `stat-label` (e.g. "Avg") sits above, `stat-sub` (e.g. "24h") sits below the value.

On mobile: the period appears **inline next to the card title** (e.g. `Avg 24h`), matching the Quartiles widget pattern. The `stat-sub` element moves to appear on the same line as `stat-label`, styled with lighter weight (`font-weight: 400`).

Implementation approach — CSS grid on the card:
```
grid-template-areas: "label sub"
                     "value value"
grid-template-columns: auto 1fr
```
- `stat-label` → grid-area `label`, baseline-aligned, no bottom margin
- `stat-sub` → grid-area `sub`, baseline-aligned, margin-top reset to 0, `display: block`
- `stat-value` → grid-area `value`, margin-top `~0.35rem`

For In Range (`stat-tir`), the `stat-value` is nested inside an extra wrapper div alongside the hidden bar. The same grid approach applies to the card; the wrapper div should be given `grid-area: value` or the card grid accounts for it.

### 2b. Value sizing

`stat-value` on these cards remains `1.75rem` — no change needed (`.stat-card:not(.stat-current) .stat-value` already sets `1.75rem`).

### 2c. Left-justify value

No change needed — values are already left-aligned by default. Removing the centered flex from previous explorations keeps them left-justified.

---

## 3. Quartiles Widget

The Quartiles widget (`stat-card stat-full`) already has `text-align: center` on `.quartile-cell` and the label is below the value in `.stat-sub`. On mobile:

- Apply the same two-row grid anchoring as Current BG (values in row 1, labels in row 2).
- No font-size changes — values remain `1.375rem`.
- The card title "Quartiles 24h" already has the period inline as a `<span>` — no change needed.

---

## What Does NOT Change

- All changes are inside `@media (max-width: 640px)` only.
- Desktop layout is untouched.
- In Range bar (`tir-bar-wrap`) remains hidden on mobile (already implemented in commit `72d0b7a`).
- Alarms tab: no changes.
- Chart card: no changes.
- Previous, High, Low cards: remain hidden on mobile (no changes).

---

## Acceptance Criteria

- [ ] Current BG header shows only "Current BG" — no time-ago on mobile.
- [ ] Current BG three columns: each shows value centered above, label centered below, at the same vertical row.
- [ ] Delta value renders at `1.75rem` on mobile.
- [ ] Trend arrow renders at `2.75rem` on mobile; `↑↑` and `↓↓` fit without overflow.
- [ ] Avg, Std Dev, CV, In Range: card title + "24h" appear on the same line at top-left.
- [ ] Avg, Std Dev, CV, In Range: value is left-justified below the label row.
- [ ] Quartiles: value and label stacked and centered in each column, labels at consistent height.
- [ ] No visual regressions on desktop (≥641px).
