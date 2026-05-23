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

	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	disp := dispatcher.New(appToken, st, cfg.Server.CallbackURL)

	for name, acctCfg := range cfg.Accounts {
		client := dexcom.New(acctCfg.DexcomUsername, acctCfg.DexcomPassword)
		p := poller.New(name, acctCfg, client, st, disp, cfg.Recipients, cfg.Health)
		go p.Run()
	}

	srv := callback.New(st, cfg.Server.CallbackPort)
	log.Fatal(srv.Start())
}
