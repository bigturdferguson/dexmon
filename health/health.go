package health

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func PingWatchdog(url string) {
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Printf("watchdog ping failed: %v", err)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	log.Printf("watchdog ping ok")
}

func FireMissedReadingsAlarm(account string, disp *dispatcher.Dispatcher, recipients map[string]config.RecipientConfig, healthCfg config.HealthConfig) {
	alarm := config.AlarmConfig{
		Name:     "Dexcom Unreachable",
		Priority: healthCfg.DexcomTimeout.Priority,
		Retry:    "5m",
		Expire:   "2h",
	}
	for _, recipientName := range healthCfg.DexcomTimeout.Recipients {
		recipientCfg, ok := recipients[recipientName]
		if !ok {
			log.Printf("health alarm: unknown recipient %q", recipientName)
			continue
		}
		req := dispatcher.SendRequest{
			Account:   account,
			AlarmName: alarm.Name,
			Recipient: recipientName,
			UserKey:   recipientCfg.PushoverUserKey,
			Message:   fmt.Sprintf("Dexcom unreachable for account %s", account),
			Alarm:     alarm,
		}
		if err := disp.Send(req, time.Now().UTC()); err != nil {
			log.Printf("health alarm dispatch to %s: %v", recipientName, err)
		}
	}
}
