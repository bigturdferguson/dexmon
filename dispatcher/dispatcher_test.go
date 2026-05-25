package dispatcher_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"dexmon/config"
	"dexmon/dispatcher"
	"dexmon/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func parsePushoverForm(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	body, _ := io.ReadAll(r.Body)
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("parse form: %v", err)
	}
	return vals
}

func TestSend_NormalPriority(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = parsePushoverForm(t, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")

	alarm := config.AlarmConfig{Name: "High", Priority: "normal", Backoff: "60m"}
	err := d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "High",
		Recipient: "brandon",
		UserKey:   "user-key-brandon",
		Message:   "High: BG 260 ↗",
		Alarm:     alarm,
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if captured.Get("priority") != "0" {
		t.Errorf("priority: got %q, want %q", captured.Get("priority"), "0")
	}
	if captured.Get("user") != "user-key-brandon" {
		t.Errorf("user key not forwarded")
	}

	state, err := st.GetAlarmState("jessica", "High", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if state.LastFiredAt == nil {
		t.Error("expected LastFiredAt set after successful Send")
	}
}

func TestSend_HighPriority(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = parsePushoverForm(t, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")

	alarm := config.AlarmConfig{Name: "Low", Priority: "high", Backoff: "30m"}
	_ = d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Message:   "Low: BG 65 ↘",
		Alarm:     alarm,
	}, time.Now().UTC())

	if captured.Get("priority") != "1" {
		t.Errorf("priority: got %q, want %q", captured.Get("priority"), "1")
	}
}

func TestSend_EmergencyPriority_SetsRetryExpireCallback(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = parsePushoverForm(t, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1, "receipt": "receipt-abc"})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "https://example.com/pushover/callback")

	alarm := config.AlarmConfig{
		Name:     "Severe Low",
		Priority: "emergency",
		Retry:    "5m",
		Expire:   "2h",
	}
	now := time.Now().UTC()
	err := d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Severe Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Message:   "Severe Low: BG 50 ↓↓",
		Alarm:     alarm,
	}, now)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if captured.Get("priority") != "2" {
		t.Errorf("priority: got %q, want 2", captured.Get("priority"))
	}
	if captured.Get("retry") != "300" {
		t.Errorf("retry: got %q, want 300", captured.Get("retry"))
	}
	if captured.Get("expire") != "7200" {
		t.Errorf("expire: got %q, want 7200", captured.Get("expire"))
	}
	if !strings.Contains(captured.Get("callback"), "pushover/callback") {
		t.Errorf("callback URL not set: %q", captured.Get("callback"))
	}

	// Verify alarm state written to store
	state, err := st.GetAlarmState("jessica", "Severe Low", "brandon")
	if err != nil {
		t.Fatalf("GetAlarmState: %v", err)
	}
	if state.LastFiredAt == nil {
		t.Error("expected LastFiredAt set")
	}
	if state.ReceiptID == nil || *state.ReceiptID != "receipt-abc" {
		t.Errorf("expected ReceiptID 'receipt-abc', got %v", state.ReceiptID)
	}
	if state.ReceiptExpiresAt == nil {
		t.Error("expected ReceiptExpiresAt set")
	} else {
		wantExpiry := now.Add(2 * time.Hour)
		if state.ReceiptExpiresAt.Before(wantExpiry.Add(-2*time.Second)) ||
			state.ReceiptExpiresAt.After(wantExpiry.Add(2*time.Second)) {
			t.Errorf("ReceiptExpiresAt: got %v, want ~%v", *state.ReceiptExpiresAt, wantExpiry)
		}
	}
}

func TestSend_LeavesStateUntouchedOnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")

	alarm := config.AlarmConfig{Name: "Low", Priority: "high"}
	err := d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Alarm:     alarm,
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for 429 response")
	}

	state, _ := st.GetAlarmState("jessica", "Low", "brandon")
	if state.LastFiredAt != nil {
		t.Error("alarm state should not be updated on API error")
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	return buf
}

func TestSend_LogsDispatch_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer srv.Close()

	buf := captureLog(t)
	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "tok", st, "")
	alarm := config.AlarmConfig{Name: "Low", Priority: "high", Backoff: "30m"}
	_ = d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Low",
		Recipient: "brandon",
		UserKey:   "ukey",
		Alarm:     alarm,
	}, time.Now().UTC())

	got := buf.String()
	if !strings.Contains(got, `[jessica] alarm "Low" fired → brandon (high)`) {
		t.Errorf("expected dispatch log line, got: %q", got)
	}
}

func TestSend_LogsDispatch_EmergencyWithReceipt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1, "receipt": "rcpt-abc"})
	}))
	defer srv.Close()

	buf := captureLog(t)
	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "tok", st, "https://example.com/cb")
	alarm := config.AlarmConfig{Name: "Urgent Low", Priority: "emergency", Retry: "5m", Expire: "2h"}
	_ = d.Send(dispatcher.SendRequest{
		Account:   "jessica",
		AlarmName: "Urgent Low",
		Recipient: "brandon",
		UserKey:   "ukey",
		Alarm:     alarm,
	}, time.Now().UTC())

	got := buf.String()
	if !strings.Contains(got, `[jessica] alarm "Urgent Low" fired → brandon (emergency, receipt rcpt-abc)`) {
		t.Errorf("expected emergency dispatch log with receipt, got: %q", got)
	}
}

func TestSend_LogsAlarmHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer srv.Close()

	st := newTestStore(t)
	d := dispatcher.NewWithAPI(srv.URL, "app-token", st, "")
	now := time.Now().UTC().Truncate(time.Second)

	alarm := config.AlarmConfig{Name: "Low", Priority: "high", Backoff: "30m"}
	err := d.Send(dispatcher.SendRequest{
		Account:   "noah",
		AlarmName: "Low",
		Recipient: "brandon",
		UserKey:   "user-key",
		Message:   "Low: BG 68 ↓",
		Alarm:     alarm,
		BGValue:   68,
	}, now)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	entries, err := st.GetAlarmHistory("noah", now.Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("GetAlarmHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].BGValue != 68 {
		t.Errorf("BGValue: got %d, want 68", entries[0].BGValue)
	}
	if entries[0].AlarmName != "Low" {
		t.Errorf("AlarmName: got %q, want %q", entries[0].AlarmName, "Low")
	}
}
