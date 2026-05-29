package dispatcher

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dexmon/config"
	"dexmon/store"
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
	BGValue   int
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
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover send: status %d", resp.StatusCode)
	}

	var result struct {
		Status  int    `json:"status"`
		Receipt string `json:"receipt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("pushover send: decode response: %w", err)
	}
	if result.Status != 1 {
		return fmt.Errorf("pushover send: API error (status %d)", result.Status)
	}

	if result.Receipt != "" {
		receiptShort := result.Receipt
		if len(receiptShort) > 8 {
			receiptShort = receiptShort[:8] + "…"
		}
		log.Printf("[%s] alarm %q fired → %s (%s, receipt %s)", req.Account, req.AlarmName, req.Recipient, req.Alarm.Priority, receiptShort)
	} else {
		log.Printf("[%s] alarm %q fired → %s (%s)", req.Account, req.AlarmName, req.Recipient, req.Alarm.Priority)
	}

	var receiptID *string
	var receiptExpiresAt *time.Time

	if req.Alarm.Priority == "emergency" && result.Receipt != "" {
		rid := result.Receipt
		receiptID = &rid
		expireDur, _ := time.ParseDuration(req.Alarm.Expire)
		t := now.Add(expireDur)
		receiptExpiresAt = &t
	}

	if err := d.store.UpdateFiredState(req.Account, req.AlarmName, req.Recipient, now, receiptID, receiptExpiresAt); err != nil {
		return err
	}
	if req.BGValue != 0 {
		if err := d.store.LogAlarmFired(req.Account, req.AlarmName, req.Recipient, now, req.BGValue); err != nil {
			log.Printf("[%s] alarm history log failed: %v", req.Account, err)
		}
	}
	return nil
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
