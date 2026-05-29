# Desktop Dashboard Redesign

**Date:** 2026-05-29  
**Branch:** feature/desktop-dashboard-redesign

## Overview

Full visual overhaul of the desktop dashboard. Mobile layout is unchanged. All desktop changes are applied in default styles (above the `@media (max-width: 640px)` block) or in a new `@media (min-width: 641px)` block.

## Layout

Two-column grid: `1fr 300px`, max-width 1280px, centered with padding.

**Left column (flex, gap 0.875rem):**
1. Current BG hero card (full width)
2. Readings chart card
3. Distribution strip card
4. Stats row (3 cards: Avg / Std Dev / CV)

**Right column (flex, gap 0.875rem):**
1. TIR donut card
2. Alarms widget (clickable → flyout)
3. Health checks card

## Header

- Time window dropdown moved into the header right side (next to theme toggle)
- The in-chart `#window-select` wrapper hidden on desktop
- `#last-updated` hidden on desktop (removed from view, element stays for mobile)
- Tab bar hidden on desktop; both panels shown as plain content

## Current BG Hero Card

Full left-column width. Layout: large BG value + unit on the left | vertical divider | 3-column meta grid on the right with: **Change** / **Trend** / **Updated**. Font sizes: BG value ~3.5rem, meta values ~1.625rem. Colors follow the existing `bg-low / bg-high / bg-normal` classes.

## Readings Chart

- Label renamed from "BG" to "READINGS" (desktop only via `#chart-title`)
- Chart y-axis positioned on the right (`position: 'right'` in Chart.js scales config)
- Time dropdown removed from chart card header on desktop (now in global header)
- Fullscreen button retained
- All existing chart rendering logic unchanged

## Distribution Strip

Shown on desktop using the existing mobile-style implementation (`renderDistributionStrip`). The `#distribution-strip` element is shown on desktop (remove `display:none` default). Label text: "DISTRIBUTION" (no time window suffix). Height 54px, same track + IQR gradient + median line visual as mobile.

## Stats Row

Three equal-width cards: Avg / Std Dev / CV. Each shows value + window label beneath. CV colored by threshold (green < 36%, orange otherwise).

## TIR Donut (right panel)

Chart.js `doughnut` chart with three segments in order: below range (red), in range (green), above range (orange). Uses `stats.time_below_range`, `stats.time_in_range`, `stats.time_above_range` from the API. Center text shows TIR percentage. Below the chart: legend rows (swatch + label + percentage). Donut destroyed/recreated on window change.

## Alarms Widget (right panel)

Clickable card. Shows up to 3 alarms with dot + name + status badge. "View all ›" in the header. Clicking anywhere on the card opens the flyout.

## Alarms Flyout

Right-edge drawer, 480px wide, slides over content. Semi-transparent overlay covers the rest (click to close). Esc key closes. Two sections: **Current Status** (alarms table: Name / Priority / Last Fired / Status) and **Recent History** (Name / BG / Time). Data is the same `alarms` and `alarm_history` arrays already in the API response.

## Health Checks Widget (right panel)

Two rows: **Dexcom API** and **Watchdog**. Each row: colored pulsing dot + name + status badge + sub-line showing age.

- **Dexcom API status**: derived from `current.recorded_at` age at render time.  
  - OK (green): < 10 min  
  - WARN (orange): 10–30 min  
  - ERROR (red): > 30 min or no current reading
- **Watchdog status**: derived from `health.watchdog` in API response.  
  - OK (green): last ping < 10 min  
  - WARN (orange): 10–30 min  
  - Not configured (muted): watchdog URL not set  
  - ERROR (red): > 30 min

## API Changes

### Stats

Add `time_below_range` and `time_above_range` float fields to `StatsJSON` (matching the work on `feature/dashboard-upgrades`).

### Health field

Add `health` object to `DashboardResponse`:

```json
{
  "health": {
    "watchdog": {
      "configured": true,
      "last_ping_at": "2026-05-29T10:00:00Z"
    }
  }
}
```

Dexcom API health is derived client-side from `current.recorded_at`. Watchdog `last_ping_at` is stored in a new `meta` KV table (`key="last_watchdog_ping"`, `value=RFC3339 timestamp`). The poller writes it after each successful ping. The handler reads it.

### Store

New `meta` table:
```sql
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
```

New methods: `SetMeta(key, value string) error` and `GetMeta(key string) (string, bool, error)`.

### Poller

After a successful `PingWatchdog` call, store the current time: `store.SetMeta("last_watchdog_ping", now.Format(time.RFC3339))`.

### Handler / Callback / Main

Thread `watchdogURL string` (from `cfg.Health.Watchdog.PingURL`) through `callback.New` → `dashboard.New` → stored on `Handler`. If the URL is non-empty, `health.watchdog.configured = true`.

## Mobile: No Changes

All new desktop styles are either default styles that the existing `@media (max-width: 640px)` block already overrides, or added in a new `@media (min-width: 641px)` block. The mobile HTML elements (`#mobile-hero`, `#distribution-strip` mobile usage, tab bar) are untouched.
