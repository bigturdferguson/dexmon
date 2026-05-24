package config_test

import (
	"os"
	"testing"

	"dexmon/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_ExpandsEnvVars(t *testing.T) {
	t.Setenv("TEST_USER_KEY", "ukey123")
	t.Setenv("DEXCOM_USER", "jess@example.com")
	t.Setenv("DEXCOM_PASS", "secret")
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = "https://example.com/cb"

[health]
  [health.dexcom_timeout]
  max_missed_readings = 3
  priority            = "emergency"
  recipients          = ["brandon"]
  [health.watchdog]
  ping_url = ""

[recipients]
  [recipients.brandon]
  pushover_user_key = "${TEST_USER_KEY}"

[accounts]
  [accounts.jessica]
  dexcom_username = "${DEXCOM_USER}"
  dexcom_password = "${DEXCOM_PASS}"
  poll_interval   = "5m"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Recipients["brandon"].PushoverUserKey != "ukey123" {
		t.Errorf("got %q, want %q", cfg.Recipients["brandon"].PushoverUserKey, "ukey123")
	}
	if cfg.Accounts["jessica"].DexcomUsername != "jess@example.com" {
		t.Errorf("got %q, want %q", cfg.Accounts["jessica"].DexcomUsername, "jess@example.com")
	}
}

func TestLoad_RejectsUnknownRecipient(t *testing.T) {
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
  [recipients.brandon]
  pushover_user_key = "ukey"

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"

  [[accounts.jessica.alarms]]
  name       = "Low"
  threshold  = 70
  direction  = "below"
  trend      = ["flat"]
  priority   = "normal"
  recipients = ["nobody"]
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for unknown recipient, got nil")
	}
}

func TestLoad_RejectsInvalidPollInterval(t *testing.T) {
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
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "not-a-duration"
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid poll_interval, got nil")
	}
}

func TestLoad_RejectsInvalidTrend(t *testing.T) {
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
  [recipients.brandon]
  pushover_user_key = "ukey"

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"

  [[accounts.jessica.alarms]]
  name       = "Low"
  threshold  = 70
  direction  = "below"
  trend      = ["Flat"]
  priority   = "normal"
  recipients = ["brandon"]
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid trend value 'Flat' (should be 'flat'), got nil")
	}
}

func TestLoad_RejectsEmergencyAlarmWithoutRetry(t *testing.T) {
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
  [recipients.brandon]
  pushover_user_key = "ukey"

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"

  [[accounts.jessica.alarms]]
  name       = "Severe Low"
  threshold  = 55
  direction  = "below"
  trend      = ["flat"]
  priority   = "emergency"
  expire     = "2h"
  recipients = ["brandon"]
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for emergency alarm without retry field, got nil")
	}
}

func TestLoad_RejectsZeroMaxMissedReadings(t *testing.T) {
	path := writeConfig(t, `
[server]
callback_port = 8080
callback_url  = ""

[health]
  [health.dexcom_timeout]
  max_missed_readings = 0
  priority            = "emergency"
  recipients          = ["brandon"]
  [health.watchdog]
  ping_url = ""

[recipients]
  [recipients.brandon]
  pushover_user_key = "ukey"

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"

  [[accounts.jessica.alarms]]
  name       = "Low"
  threshold  = 70
  direction  = "below"
  trend      = ["flat"]
  priority   = "normal"
  recipients = ["brandon"]
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for max_missed_readings=0 with recipients configured, got nil")
	}
}

func TestLoad_RejectsEmptyTrend(t *testing.T) {
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
  [recipients.brandon]
  pushover_user_key = "ukey"

[accounts]
  [accounts.jessica]
  dexcom_username = "u"
  dexcom_password = "p"
  poll_interval   = "5m"

  [[accounts.jessica.alarms]]
  name       = "Low"
  threshold  = 70
  direction  = "below"
  trend      = []
  priority   = "normal"
  recipients = ["brandon"]
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for empty trend list, got nil")
	}
}

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
