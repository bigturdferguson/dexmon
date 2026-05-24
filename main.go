package main

import (
	"flag"
	"log"
	"os"

	"dexmon/callback"
	"dexmon/config"
	"dexmon/dexcom"
	"dexmon/dispatcher"
	"dexmon/poller"
	"dexmon/store"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	dbPath := flag.String("db", "dexmon.db", "path to SQLite database")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	appToken := os.Getenv("PUSHOVER_APP_TOKEN")
	if appToken == "" {
		log.Fatal("PUSHOVER_APP_TOKEN environment variable is required")
	}

	logStartup(cfg)

	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	disp := dispatcher.New(appToken, st, cfg.Server.CallbackURL)

	for name, acctCfg := range cfg.Accounts {
		client := dexcom.New(acctCfg.DexcomUsername, acctCfg.DexcomPassword)
		p := poller.New(name, acctCfg, client, st, disp, cfg.Recipients, cfg.Health)
		go p.Run()
	}

	// Extract the single monitored account for the dashboard.
	if len(cfg.Accounts) == 0 {
		log.Fatal("config: at least one account is required")
	}
	if len(cfg.Accounts) > 1 {
		log.Fatal("config: dashboard supports only one account; multiple accounts are not yet supported")
	}
	var accountName string
	var accountAlarms []config.AlarmConfig
	for name, acct := range cfg.Accounts {
		accountName = name
		accountAlarms = acct.Alarms
		break
	}

	srv := callback.New(st, cfg.Server.CallbackPort, accountName, accountAlarms, cfg.Recipients)
	if err := srv.Start(); err != nil {
		st.Close()
		log.Fatal(err)
	}
}

func logStartup(cfg *config.Config) {
	if cfg.Server.CallbackURL != "" {
		log.Printf("config: callback URL: %s", cfg.Server.CallbackURL)
	} else {
		log.Printf("config: callback URL: (not set — emergency callbacks disabled)")
	}
	for name, acct := range cfg.Accounts {
		log.Printf("config: account %q polling every %s, %d alarms", name, acct.PollInterval, len(acct.Alarms))
	}
	for name := range cfg.Recipients {
		log.Printf("config: recipient %q configured", name)
	}
	if cfg.Health.Watchdog.PingURL != "" {
		log.Printf("config: watchdog ping URL: %s", cfg.Health.Watchdog.PingURL)
	} else {
		log.Printf("config: watchdog ping: (not set)")
	}
}
