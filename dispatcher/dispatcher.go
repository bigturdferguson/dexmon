package dispatcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dexmon/config"
	"dexmon/store"
	"dexmon/types"
)

const defaultPushoverAPI = "https://api.pushover.net/1/messages.json"

type Dispatcher struct {
	apiURL      string
	appToken    string
	store       *store.Store
	callbackURL string
	http        *http.Client
}

func New(appToken string, store *store.Store, callbackURL string) *Dispatcher {
	return NewWithAPI(defaultPushoverAPI, appToken, store, callbackURL)
}

func NewWithAPI(apiURL, appToken string, store *store.Store, callbackURL string) *Dispatcher {
	return &Dispatcher{
		apiURL:      apiURL,
		appToken:    appToken,
		store:       store,
		callbackURL: callbackURL,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

type SendRequest struct {
	Account   string
	AlarmName string
	Recipient string
	UserKey   string
	Message   string
	Alarm     config.AlarmConfig
}

func (d *Dispatcher) Send(req SendRequest, now time.Time) error {
	priority := priorityCode(req.Alarm.Priority)

	form := url.Values{
		"token":    {d.appToken},
		"user":     {req.UserKey},
		"message":  {req.Message},
		"priority": {fmt.Sprintf("%d", priority)},
	}

	if req.Alarm.Priority == "emergency" {
		form.Set("retry", durationSeconds(req.Alarm.Retry))
		form.Set("expire", durationSeconds(req.Alarm.Expire))
		if d.callbackURL != "" {
			form.Set("callback", d.callbackURL)
		}
	}

	resp, err := d.http.Post(d.apiURL, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("pushover send: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status  int    `json:"status"`
		Receipt string `json:"receipt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("pushover send: decode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover send: status %d", resp.StatusCode)
	}

	lastFired := now
	state := types.AlarmState{
		Account:     req.Account,
		AlarmName:   req.AlarmName,
		Recipient:   req.Recipient,
		LastFiredAt: &lastFired,
	}

	if req.Alarm.Priority == "emergency" && result.Receipt != "" {
		rid := result.Receipt
		state.ReceiptID = &rid
		expireDur, _ := time.ParseDuration(req.Alarm.Expire)
		t := now.Add(expireDur)
		state.ReceiptExpiresAt = &t
	}

	return d.store.UpsertAlarmState(state)
}

func priorityCode(p string) int {
	switch p {
	case "emergency":
		return 2
	case "high":
		return 1
	default:
		return 0
	}
}

func durationSeconds(s string) string {
	d, _ := time.ParseDuration(s)
	return fmt.Sprintf("%d", int(d.Seconds()))
}
