# Per-Account Target Range Design

**Date:** 2026-05-24
**Status:** Approved

## Overview

Each Dexcom account gets two optional config fields — `target_low` and `target_high` — that drive dashboard display only. The range controls BG color coding on stat widgets, chart point colors, and the shaded reference band on the graph. Alarm thresholds remain independent.

---

## Config

Two optional fields on `AccountConfig`:

```toml
[accounts.noah]
target_low  = 70   # optional, defaults to 70
target_high = 180  # optional, defaults to 180
```

**Defaults:** If either field is omitted (zero value after TOML decode), `validate()` sets `TargetLow = 70` and `TargetHigh = 180`.

**Validation (fail startup if violated):**
- `target_low > 0`
- `target_high > 0`
- `target_low < target_high`

**Go struct change** (`config/config.go`):

```go
type AccountConfig struct {
    DexcomUsername string        `toml:"dexcom_username"`
    DexcomPassword string        `toml:"dexcom_password"`
    PollInterval   string        `toml:"poll_interval"`
    TargetLow      int           `toml:"target_low"`
    TargetHigh     int           `toml:"target_high"`
    Alarms         []AlarmConfig `toml:"alarms"`
}
```

---

## Backend

### Signature changes

`dashboard.New()` gains two parameters:

```go
func New(st *store.Store, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int) *Handler
```

`callback.New()` gains the same two:

```go
func New(st *store.Store, port int, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int) *Server
```

### Handler struct

```go
type Handler struct {
    store      *store.Store
    account    string
    alarms     []config.AlarmConfig
    recipients map[string]config.RecipientConfig
    targetLow  int
    targetHigh int
}
```

### API response

`DashboardResponse` gains a `Target` field:

```go
type TargetJSON struct {
    Low  int `json:"low"`
    High int `json:"high"`
}

type DashboardResponse struct {
    Account  string        `json:"account"`
    AsOf     time.Time     `json:"as_of"`
    Target   TargetJSON    `json:"target"`
    Current  *ReadingJSON  `json:"current"`
    Previous *ReadingJSON  `json:"previous"`
    Stats    StatsJSON     `json:"stats"`
    Readings []ReadingJSON `json:"readings"`
    Alarms   []AlarmJSON   `json:"alarms"`
}
```

Populated in `serveAPI`:

```go
resp := DashboardResponse{
    Account: h.account,
    AsOf:    now,
    Target:  TargetJSON{Low: h.targetLow, High: h.targetHigh},
    ...
}
```

### `main.go` extraction

```go
var accountName string
var accountAlarms []config.AlarmConfig
var targetLow, targetHigh int
for name, acct := range cfg.Accounts {
    accountName = name
    accountAlarms = acct.Alarms
    targetLow = acct.TargetLow
    targetHigh = acct.TargetHigh
    break
}

srv := callback.New(st, cfg.Server.CallbackPort, accountName, accountAlarms, cfg.Recipients, targetLow, targetHigh)
```

---

## Frontend

Four hardcoded references to `70` and `180` in `dashboard/static/index.html` are replaced with values from `data.target`.

### `bgClass(v)` — stat widget color

```js
function bgClass(v) {
  if (v < lastData.target.low)  return 'bg-low';
  if (v > lastData.target.high) return 'bg-high';
  return 'bg-normal';
}
```

### `bgColor(v)` — chart point color

```js
function bgColor(v) {
  if (v < lastData.target.low)  return cssVar('--low');
  if (v > lastData.target.high) return cssVar('--high');
  return cssVar('--normal');
}
```

### `renderChart()` — reference lines and shaded band

```js
const lowLine  = Array(readings.length).fill(lastData.target.low);
const highLine = Array(readings.length).fill(lastData.target.high);
```

The chart update branch also uses `lastData.target.low` / `lastData.target.high`:

```js
chart.data.datasets[1].data = Array(readings.length).fill(lastData.target.low);
chart.data.datasets[2].data = Array(readings.length).fill(lastData.target.high);
```

`bgClass` and `bgColor` currently use hardcoded `70`/`180`. After this change they will reference `lastData.target.low` / `lastData.target.high`. This is safe because `lastData` is assigned (`lastData = d`) before `renderWidgets` and `renderChart` are called — these functions are never invoked until after a successful fetch.

---

## Tests

### `config/config_test.go`
- `target_low` and `target_high` omitted → defaults to 70 and 180
- Explicit values round-trip correctly
- `target_low >= target_high` → validation error
- `target_low <= 0` or `target_high <= 0` → validation error

### `dashboard/handler_test.go`
- `GET /api/dashboard` response includes `"target": {"low": N, "high": M}` with the values passed to `New()`
- Existing tests: update `dashboard.New()` calls to pass `70, 180`

### `callback/server_test.go`
- Update all `callback.New()` calls to pass `70, 180` (the callback tests don't exercise the dashboard handler, but using the standard defaults keeps the calls semantically correct)

---

## Out of Scope

- Dashboard time range filter
- Using the target range to drive alarm thresholds
- Units (mg/dL only)
