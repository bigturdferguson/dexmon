package poller

import (
	"fmt"
	"log"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
	"dexmon/evaluator"
	"dexmon/health"
	"dexmon/store"
	"dexmon/types"
)

// Fetcher is satisfied by *dexcom.Client and test mocks.
type Fetcher interface {
	Login() error
	FetchLatest(account string) (*types.Reading, error)
}

type Poller struct {
	accountName      string
	cfg              config.AccountConfig
	fetcher          Fetcher
	store            *store.Store
	disp             *dispatcher.Dispatcher
	recipients       map[string]config.RecipientConfig
	healthCfg        config.HealthConfig
	missCount        int
	healthAlarmFired bool
}

func New(accountName string, cfg config.AccountConfig, fetcher Fetcher, st *store.Store, disp *dispatcher.Dispatcher, recipients map[string]config.RecipientConfig, healthCfg config.HealthConfig) *Poller {
	return &Poller{
		accountName: accountName,
		cfg:         cfg,
		fetcher:     fetcher,
		store:       st,
		disp:        disp,
		recipients:  recipients,
		healthCfg:   healthCfg,
	}
}

func (p *Poller) Run() {
	if err := p.fetcher.Login(); err != nil {
		log.Printf("[%s] initial login failed: %v", p.accountName, err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -30)
	if err := p.store.PruneReadings(p.accountName, cutoff); err != nil {
		log.Printf("[%s] prune readings: %v", p.accountName, err)
	}
	interval, _ := time.ParseDuration(p.cfg.PollInterval)
	p.Tick()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		p.Tick()
	}
}

// Tick executes one poll cycle. Exported for testing.
func (p *Poller) Tick() {
	now := time.Now().UTC()
	reading, err := p.fetcher.FetchLatest(p.accountName)
	if err != nil {
		p.missCount++
		log.Printf("[%s] fetch error (%d consecutive): %v", p.accountName, p.missCount, err)
		if p.missCount >= p.healthCfg.DexcomTimeout.MaxMissedReadings && !p.healthAlarmFired {
			health.FireMissedReadingsAlarm(p.accountName, p.disp, p.recipients, p.healthCfg)
			p.healthAlarmFired = true
		}
		return
	}
	if reading == nil {
		log.Printf("[%s] poll: no reading returned", p.accountName)
		return
	}

	exists, err := p.store.HasReading(p.accountName, reading.RecordedAt)
	if err != nil {
		log.Printf("[%s] check reading: %v", p.accountName, err)
		return
	}
	if exists {
		log.Printf("[%s] poll: BG %d %s %s (already seen)", p.accountName, reading.Value, trendArrow(reading.Trend), time.Since(reading.RecordedAt).Round(time.Second))
		return
	}

	if err := p.store.InsertReading(*reading); err != nil {
		log.Fatalf("[%s] insert reading (store fatal): %v", p.accountName, err)
	}

	log.Printf("[%s] poll: BG %d %s %s", p.accountName, reading.Value, trendArrow(reading.Trend), time.Since(reading.RecordedAt).Round(time.Second))

	p.missCount = 0
	p.healthAlarmFired = false

	if url := p.healthCfg.Watchdog.PingURL; url != "" {
		health.PingWatchdog(url)
	}

	toFire, toRearm, err := evaluator.Evaluate(p.accountName, p.cfg.Alarms, *reading, p.store, now)
	if err != nil {
		log.Printf("[%s] evaluate: %v", p.accountName, err)
		return
	}

	for _, result := range toRearm {
		if err := p.store.ClearAlarmRearm(p.accountName, result.AlarmName, result.Recipient); err != nil {
			log.Printf("[%s] clear alarm rearm: %v", p.accountName, err)
		} else {
			log.Printf("[%s] alarm %q rearmed for %s", p.accountName, result.AlarmName, result.Recipient)
		}
	}

	for _, result := range toFire {
		recipientCfg := p.recipients[result.Recipient]
		req := dispatcher.SendRequest{
			Account:   p.accountName,
			AlarmName: result.AlarmName,
			Recipient: result.Recipient,
			UserKey:   recipientCfg.PushoverUserKey,
			Message:   formatMessage(*reading, result.Alarm),
			Alarm:     result.Alarm,
		}
		if err := p.disp.Send(req, now); err != nil {
			log.Printf("[%s] dispatch to %s: %v", p.accountName, result.Recipient, err)
		}
	}
}

func formatMessage(r types.Reading, alarm config.AlarmConfig) string {
	return fmt.Sprintf("%s: BG %d %s", alarm.Name, r.Value, trendArrow(r.Trend))
}

func trendArrow(t types.Trend) string {
	switch t {
	case types.TrendDoubleUp:
		return "↑↑"
	case types.TrendSingleUp:
		return "↑"
	case types.TrendFortyFiveUp:
		return "↗"
	case types.TrendFlat:
		return "→"
	case types.TrendFortyFiveDown:
		return "↘"
	case types.TrendSingleDown:
		return "↓"
	case types.TrendDoubleDown:
		return "↓↓"
	default:
		return ""
	}
}
