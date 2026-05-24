# Per-Account Target Range Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional `target_low` and `target_high` fields to each account config that drive BG color coding and chart reference band on the dashboard (display only, not alarm thresholds).

**Architecture:** Config layer adds two `int` fields with zero-value defaults applied in `validate()`. The dashboard `Handler` carries the values and includes them in every API response inside a `Target` field. The frontend replaces four hardcoded `70`/`180` literals with `lastData.target.low` / `lastData.target.high`.

**Tech Stack:** Go (config, handler), TOML (config file), vanilla JS (frontend)

---

### Task 1: Config — Add `TargetLow` / `TargetHigh` to `AccountConfig`

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `config/config_test.go`:

```go
func TestLoad_TargetRange_Defaults(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]

[accounts]
  [accounts.noah]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	acct := cfg.Accounts["noah"]
	if acct.TargetLow != 70 {
		t.Errorf("expected TargetLow=70, got %d", acct.TargetLow)
	}
	if acct.TargetHigh != 180 {
		t.Errorf("expected TargetHigh=180, got %d", acct.TargetHigh)
	}
}

func TestLoad_TargetRange_ExplicitValues(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]

[accounts]
  [accounts.noah]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"
  target_low      = 80
  target_high     = 140
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	acct := cfg.Accounts["noah"]
	if acct.TargetLow != 80 {
		t.Errorf("expected TargetLow=80, got %d", acct.TargetLow)
	}
	if acct.TargetHigh != 140 {
		t.Errorf("expected TargetHigh=140, got %d", acct.TargetHigh)
	}
}

func TestLoad_TargetRange_LowNotLessThanHigh(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]

[accounts]
  [accounts.noah]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"
  target_low      = 180
  target_high     = 70
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for target_low >= target_high, got nil")
	}
}

func TestLoad_TargetRange_NegativeLow(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]

[accounts]
  [accounts.noah]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"
  target_low      = -1
  target_high     = 180
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for target_low <= 0, got nil")
	}
}

func TestLoad_TargetRange_NegativeHigh(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = []
  [health.watchdog]
  ping_url = ""

[recipients]

[accounts]
  [accounts.noah]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"
  target_low      = 70
  target_high     = -1
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for target_high <= 0, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/brandon/Projects/dexmon && go test ./config/ -run 'TestLoad_TargetRange' -v
```

Expected: FAIL — `cfg.Accounts["noah"].TargetLow` does not exist yet.

- [ ] **Step 3: Add `TargetLow` and `TargetHigh` to `AccountConfig` in `config/config.go`**

Change the `AccountConfig` struct:

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

- [ ] **Step 4: Apply defaults and validate in `validate()` in `config/config.go`**

Inside the `for name, acct := range cfg.Accounts` loop, add this block immediately after parsing `PollInterval` (before alarm validation). Because map values in Go are not addressable, you must copy the struct, modify it, and write it back before using the updated values:

```go
		if acct.TargetLow == 0 {
			acct.TargetLow = 70
		}
		if acct.TargetHigh == 0 {
			acct.TargetHigh = 180
		}
		cfg.Accounts[name] = acct
		if acct.TargetLow <= 0 {
			return fmt.Errorf("account %q: target_low must be > 0", name)
		}
		if acct.TargetHigh <= 0 {
			return fmt.Errorf("account %q: target_high must be > 0", name)
		}
		if acct.TargetLow >= acct.TargetHigh {
			return fmt.Errorf("account %q: target_low must be less than target_high", name)
		}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/brandon/Projects/dexmon && go test ./config/ -v
```

Expected: all config tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/brandon/Projects/dexmon && git add config/config.go config/config_test.go
git commit -m "feat: add target_low/target_high to AccountConfig with defaults and validation"
```

---

### Task 2: Dashboard handler — expose `Target` in API response

**Files:**
- Modify: `dashboard/handler.go`
- Modify: `dashboard/handler_test.go`

- [ ] **Step 1: Write the failing test**

Add to `dashboard/handler_test.go`:

```go
func TestDashboardAPI_TargetRange(t *testing.T) {
	s := newTestStore(t)
	h := dashboard.New(s, "noah", nil, nil, 80, 140)
	w := get(t, h, "/api/dashboard")

	var resp dashboard.DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Target.Low != 80 {
		t.Errorf("expected Target.Low=80, got %d", resp.Target.Low)
	}
	if resp.Target.High != 140 {
		t.Errorf("expected Target.High=140, got %d", resp.Target.High)
	}
}
```

- [ ] **Step 2: Run the new test to verify it fails**

```bash
cd /home/brandon/Projects/dexmon && go test ./dashboard/ -run 'TestDashboardAPI_TargetRange' -v
```

Expected: FAIL — compile error, `New` doesn't accept 6 args and `DashboardResponse.Target` does not exist.

- [ ] **Step 3: Update `dashboard/handler.go`**

Add `TargetJSON` type and `Target` field to `DashboardResponse`:

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

Update `Handler` struct to carry target fields:

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

Update `New()` to accept and store them:

```go
func New(st *store.Store, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int) *Handler {
	return &Handler{store: st, account: account, alarms: alarms, recipients: recipients, targetLow: targetLow, targetHigh: targetHigh}
}
```

Populate `Target` in `serveAPI` — update the `resp := DashboardResponse{...}` literal:

```go
resp := DashboardResponse{
	Account:  h.account,
	AsOf:     now,
	Target:   TargetJSON{Low: h.targetLow, High: h.targetHigh},
	Stats:    StatsJSON{High: maxVal, Low: minVal, Avg: avgVal},
	Readings: toReadingJSON(readings),
	Alarms:   h.buildAlarmList(now),
}
```

- [ ] **Step 4: Update all existing `dashboard.New()` calls in `dashboard/handler_test.go` to pass `70, 180`**

There are 8 calls. Change every `dashboard.New(s, "noah", ...)` to append `70, 180` as the last two arguments:

| Line | Before | After |
|------|--------|-------|
| 36 | `dashboard.New(s, "noah", nil, nil)` | `dashboard.New(s, "noah", nil, nil, 70, 180)` |
| 85 | `dashboard.New(s, "noah", nil, nil)` | `dashboard.New(s, "noah", nil, nil, 70, 180)` |
| 115 | `dashboard.New(s, "noah", alarms, recipients)` | `dashboard.New(s, "noah", alarms, recipients, 70, 180)` |
| 149 | `dashboard.New(s, "noah", alarms, nil)` | `dashboard.New(s, "noah", alarms, nil, 70, 180)` |
| 178 | `dashboard.New(s, "noah", alarms, nil)` | `dashboard.New(s, "noah", alarms, nil, 70, 180)` |
| 204 | `dashboard.New(s, "noah", alarms, nil)` | `dashboard.New(s, "noah", alarms, nil, 70, 180)` |
| 239 | `dashboard.New(s, "noah", alarms, nil)` | `dashboard.New(s, "noah", alarms, nil, 70, 180)` |
| 254 | `dashboard.New(s, "noah", nil, nil)` | `dashboard.New(s, "noah", nil, nil, 70, 180)` |

- [ ] **Step 5: Run all dashboard tests to verify they pass**

```bash
cd /home/brandon/Projects/dexmon && go test ./dashboard/ -v
```

Expected: all dashboard tests PASS including `TestDashboardAPI_TargetRange`.

- [ ] **Step 6: Commit**

```bash
cd /home/brandon/Projects/dexmon && git add dashboard/handler.go dashboard/handler_test.go
git commit -m "feat: add TargetJSON to DashboardResponse; pass targetLow/targetHigh through Handler"
```

---

### Task 3: Wire-up — thread target values through `callback.New()` and `main.go`

**Files:**
- Modify: `callback/server.go`
- Modify: `callback/server_test.go`
- Modify: `main.go`

- [ ] **Step 1: Verify the build is broken (expected)**

```bash
cd /home/brandon/Projects/dexmon && go build ./...
```

Expected: compile errors in `callback/server.go` and `main.go` because `dashboard.New` now requires 6 args.

- [ ] **Step 2: Update `callback/server.go`**

Change `New()` to accept and forward `targetLow, targetHigh int`:

```go
func New(st *store.Store, port int, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int) *Server {
	s := &Server{store: st, port: port, mux: http.NewServeMux()}
	dash := dashboard.New(st, account, alarms, recipients, targetLow, targetHigh)
	s.mux.Handle("GET /", dash)
	s.mux.Handle("GET /api/dashboard", dash)
	s.mux.HandleFunc("POST /pushover/callback", s.handleCallback)
	return s
}
```

- [ ] **Step 3: Update `main.go` to extract target values and pass them**

In the account-extraction block, add `targetLow` and `targetHigh`:

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

- [ ] **Step 4: Update all `callback.New()` calls in `callback/server_test.go` to pass `70, 180`**

There are 5 occurrences of `callback.New(st, 0, "", nil, nil)`. Change each one to:

```go
callback.New(st, 0, "", nil, nil, 70, 180)
```

Lines to update: 43, 80, 128, 150, 193.

- [ ] **Step 5: Verify the build and all tests pass**

```bash
cd /home/brandon/Projects/dexmon && go build ./... && go test ./...
```

Expected: build succeeds, all tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/brandon/Projects/dexmon && git add callback/server.go callback/server_test.go main.go
git commit -m "feat: thread targetLow/targetHigh through callback.New and main"
```

---

### Task 4: Frontend — replace hardcoded 70/180 with `lastData.target`

**Files:**
- Modify: `dashboard/static/index.html`

- [ ] **Step 1: Locate the four hardcoded references**

```bash
grep -n "70\|180" /home/brandon/Projects/dexmon/dashboard/static/index.html | grep -v "font-size\|letter-spacing\|font-weight\|--high\|0\.08"
```

Expected output (line numbers may vary slightly):

```
258:      if (v < 70) return 'bg-low';
259:      if (v > 180) return 'bg-high';
263:      if (v < 70) return cssVar('--low');
264:      if (v > 180) return cssVar('--high');
312:      const lowLine   = Array(readings.length).fill(70);
313:      const highLine  = Array(readings.length).fill(180);
```

- [ ] **Step 2: Replace the four hardcoded values**

In `bgClass(v)` replace:
```js
      if (v < 70) return 'bg-low';
      if (v > 180) return 'bg-high';
```
with:
```js
      if (v < lastData.target.low)  return 'bg-low';
      if (v > lastData.target.high) return 'bg-high';
```

In `bgColor(v)` replace:
```js
      if (v < 70) return cssVar('--low');
      if (v > 180) return cssVar('--high');
```
with:
```js
      if (v < lastData.target.low)  return cssVar('--low');
      if (v > lastData.target.high) return cssVar('--high');
```

In `renderChart(readings)` replace:
```js
      const lowLine   = Array(readings.length).fill(70);
      const highLine  = Array(readings.length).fill(180);
```
with:
```js
      const lowLine   = Array(readings.length).fill(lastData.target.low);
      const highLine  = Array(readings.length).fill(lastData.target.high);
```

The chart update branch (the `if (chart)` path that sets `chart.data.datasets[1].data = lowLine`) already references the `lowLine`/`highLine` variables — no additional change needed there.

These references to `lastData.target` are safe because `lastData` is assigned (`lastData = d`) before `renderWidgets` and `renderChart` are called — these functions are never invoked until after a successful fetch.

- [ ] **Step 3: Run all tests to confirm no regressions**

```bash
cd /home/brandon/Projects/dexmon && go test ./...
```

Expected: all tests PASS (frontend change has no Go test coverage — correctness verified by manual inspection).

- [ ] **Step 4: Commit**

```bash
cd /home/brandon/Projects/dexmon && git add dashboard/static/index.html
git commit -m "feat: replace hardcoded 70/180 in dashboard JS with lastData.target.low/high"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `TargetLow`/`TargetHigh` fields on `AccountConfig` | Task 1 |
| Defaults to 70/180 when omitted | Task 1 |
| Validation: > 0, low < high | Task 1 |
| Config tests (defaults, round-trip, validation errors) | Task 1 |
| `TargetJSON` type | Task 2 |
| `Target` field on `DashboardResponse` | Task 2 |
| `Handler` carries `targetLow`/`targetHigh` | Task 2 |
| `dashboard.New()` gains two params | Task 2 |
| Existing dashboard test calls updated | Task 2 |
| New test for Target field | Task 2 |
| `callback.New()` gains two params | Task 3 |
| `main.go` extracts and passes target values | Task 3 |
| Callback test calls updated | Task 3 |
| `bgClass(v)` uses `lastData.target` | Task 4 |
| `bgColor(v)` uses `lastData.target` | Task 4 |
| `renderChart()` uses `lastData.target` | Task 4 |

All spec requirements covered. No gaps found.

**Placeholder scan:** No TBDs, no vague steps, all code blocks complete.

**Type consistency:** `TargetJSON{Low, High int}` defined in Task 2 and referenced identically across all tasks. `targetLow, targetHigh int` parameter names consistent through Tasks 2→3→4.
