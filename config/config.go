package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
	"dexmon/types"
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
	TargetLow      int           `toml:"target_low"`
	TargetHigh     int           `toml:"target_high"`
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
		for _, alarm := range acct.Alarms {
			if alarm.Direction != "above" && alarm.Direction != "below" {
				return fmt.Errorf("account %q, alarm %q: direction must be 'above' or 'below'", name, alarm.Name)
			}
			if alarm.Priority != "emergency" && alarm.Priority != "high" && alarm.Priority != "normal" {
				return fmt.Errorf("account %q, alarm %q: priority must be emergency/high/normal", name, alarm.Name)
			}
			for _, field := range []struct{ name, val string }{
				{"retry", alarm.Retry}, {"expire", alarm.Expire}, {"backoff", alarm.Backoff},
			} {
				if field.val != "" {
					if _, err := time.ParseDuration(field.val); err != nil {
						return fmt.Errorf("account %q, alarm %q: invalid %s %q: %w", name, alarm.Name, field.name, field.val, err)
					}
				}
			}
			if alarm.Priority == "emergency" {
				if alarm.Retry == "" {
					return fmt.Errorf("account %q, alarm %q: emergency alarms require a retry value", name, alarm.Name)
				}
				if alarm.Expire == "" {
					return fmt.Errorf("account %q, alarm %q: emergency alarms require an expire value", name, alarm.Name)
				}
			}
			for _, r := range alarm.Recipients {
				if _, ok := cfg.Recipients[r]; !ok {
					return fmt.Errorf("account %q, alarm %q: unknown recipient %q", name, alarm.Name, r)
				}
			}
			if len(alarm.Trend) == 0 {
				return fmt.Errorf("account %q, alarm %q: trend list must not be empty", name, alarm.Name)
			}
			validTrends := map[string]bool{
				string(types.TrendDoubleUp):       true,
				string(types.TrendSingleUp):       true,
				string(types.TrendFortyFiveUp):    true,
				string(types.TrendFlat):           true,
				string(types.TrendFortyFiveDown):  true,
				string(types.TrendSingleDown):     true,
				string(types.TrendDoubleDown):     true,
				string(types.TrendNotComputable):  true,
				string(types.TrendRateOutOfRange): true,
				string(types.TrendNone):           true,
			}
			for _, t := range alarm.Trend {
				if !validTrends[t] {
					return fmt.Errorf("account %q, alarm %q: invalid trend value %q", name, alarm.Name, t)
				}
			}
		}
	}
	for _, r := range cfg.Health.DexcomTimeout.Recipients {
		if _, ok := cfg.Recipients[r]; !ok {
			return fmt.Errorf("health.dexcom_timeout: unknown recipient %q", r)
		}
	}
	if p := cfg.Health.DexcomTimeout.Priority; p != "" && p != "emergency" && p != "high" && p != "normal" {
		return fmt.Errorf("health.dexcom_timeout: priority must be emergency/high/normal, got %q", p)
	}
	if len(cfg.Health.DexcomTimeout.Recipients) > 0 && cfg.Health.DexcomTimeout.MaxMissedReadings <= 0 {
		return fmt.Errorf("health.dexcom_timeout: max_missed_readings must be > 0")
	}
	return nil
}
