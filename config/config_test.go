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
	f.WriteString(content)
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
