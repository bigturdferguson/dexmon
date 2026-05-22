package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server     ServerConfig               `toml:"server"`
	Health     HealthConfig               `toml:"health"`
	Recipients map[string]RecipientConfig `toml:"recipients"`
	Accounts   map[string]AccountConfig   `toml:"accounts"`
}

type ServerConfig struct {
	CallbackPort int    `toml:"callback_port"`
	CallbackURL  string `toml:"callback_url"`
}

type HealthConfig struct {
	DexcomTimeout DexcomTimeoutConfig `toml:"dexcom_timeout"`
	Watchdog      WatchdogConfig      `toml:"watchdog"`
}

type DexcomTimeoutConfig struct {
	MaxMissedReadings int      `toml:"max_missed_readings"`
	Priority          string   `toml:"priority"`
	Recipients        []string `toml:"recipients"`
}

type WatchdogConfig struct {
	PingURL string `toml:"ping_url"`
}

type RecipientConfig struct {
	PushoverUserKey string `toml:"pushover_user_key"`
}

type AccountConfig struct {
	DexcomUsername string        `toml:"dexcom_username"`
	DexcomPassword string        `toml:"dexcom_password"`
	PollInterval   string        `toml:"poll_interval"`
	Alarms         []AlarmConfig `toml:"alarms"`
}

type AlarmConfig struct {
	Name            string   `toml:"name"`
	Threshold       int      `toml:"threshold"`
	Direction       string   `toml:"direction"`
	Trend           []string `toml:"trend"`
	Priority        string   `toml:"priority"`
	Retry           string   `toml:"retry"`
	Expire          string   `toml:"expire"`
	Backoff         string   `toml:"backoff"`
	RearmOnRecovery bool     `toml:"rearm_on_recovery"`
	Recipients      []string `toml:"recipients"`
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	expanded := envVarRe.ReplaceAllStringFunc(string(data), func(match string) string {
		key := envVarRe.FindStringSubmatch(match)[1]
		return os.Getenv(key)
	})
	var cfg Config
	if _, err := toml.Decode(expanded, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

func validate(cfg *Config) error {
	for name, acct := range cfg.Accounts {
		if acct.DexcomUsername == "" {
			return fmt.Errorf("account %q: dexcom_username required", name)
		}
		if acct.DexcomPassword == "" {
			return fmt.Errorf("account %q: dexcom_password required", name)
		}
		if _, err := time.ParseDuration(acct.PollInterval); err != nil {
			return fmt.Errorf("account %q: invalid poll_interval %q: %w", name, acct.PollInterval, err)
		}
		for _, alarm := range acct.Alarms {
			if alarm.Direction != "above" && alarm.Direction != "below" {
				return fmt.Errorf("account %q, alarm %q: direction must be 'above' or 'below'", name, alarm.Name)
			}
			if alarm.Priority != "emergency" && alarm.Priority != "high" && alarm.Priority != "normal" {
				return fmt.Errorf("account %q, alarm %q: priority must be emergency/high/normal", name, alarm.Name)
			}
			for _, r := range alarm.Recipients {
				if _, ok := cfg.Recipients[r]; !ok {
					return fmt.Errorf("account %q, alarm %q: unknown recipient %q", name, alarm.Name, r)
				}
			}
		}
	}
	return nil
}
