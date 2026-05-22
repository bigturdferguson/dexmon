package dexcom_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dexmon/dexcom"
	"dexmon/types"
)

func TestLogin_StoresSessionID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ShareWebServices/Services/General/LoginPublisherAccountByName" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode("session-abc-123")
	}))
	defer srv.Close()

	c := dexcom.NewWithBase("user", "pass", srv.URL+"/ShareWebServices/Services")
	if err := c.Login(); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !c.HasSession() {
		t.Error("expected session to be stored after Login")
	}
}

func TestFetchLatest_ReturnsReading(t *testing.T) {
	loginCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ShareWebServices/Services/General/LoginPublisherAccountByName":
			loginCalled = true
			json.NewEncoder(w).Encode("session-xyz")
		case "/ShareWebServices/Services/Publisher/ReadPublisherLatestGlucoseValues":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"WT": "/Date(1700000000000)/", "Value": 92, "Trend": "Flat"},
			})
		}
	}))
	defer srv.Close()

	c := dexcom.NewWithBase("user", "pass", srv.URL+"/ShareWebServices/Services")
	reading, err := c.FetchLatest("jessica")
	if err != nil {
		t.Fatalf("FetchLatest: %v", err)
	}
	if !loginCalled {
		t.Error("expected Login to be called automatically")
	}
	if reading == nil {
		t.Fatal("expected reading, got nil")
	}
	if reading.Value != 92 {
		t.Errorf("Value: got %d, want 92", reading.Value)
	}
	if reading.Trend != types.TrendFlat {
		t.Errorf("Trend: got %q, want %q", reading.Trend, types.TrendFlat)
	}
	wantTime := time.UnixMilli(1700000000000).UTC()
	if !reading.RecordedAt.Equal(wantTime) {
		t.Errorf("RecordedAt: got %v, want %v", reading.RecordedAt, wantTime)
	}
}

func TestFetchLatest_ReauthOnSessionExpiry(t *testing.T) {
	loginCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ShareWebServices/Services/General/LoginPublisherAccountByName":
			loginCount++
			json.NewEncoder(w).Encode("new-session")
		case "/ShareWebServices/Services/Publisher/ReadPublisherLatestGlucoseValues":
			if loginCount < 2 {
				// Simulate expired session
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"WT": "/Date(1700000000000)/", "Value": 100, "Trend": "SingleUp"},
			})
		}
	}))
	defer srv.Close()

	c := dexcom.NewWithBase("user", "pass", srv.URL+"/ShareWebServices/Services")
	reading, err := c.FetchLatest("jessica")
	if err != nil {
		t.Fatalf("FetchLatest after reauth: %v", err)
	}
	if loginCount != 2 {
		t.Errorf("expected 2 logins (initial + reauth), got %d", loginCount)
	}
	if reading == nil || reading.Value != 100 {
		t.Errorf("unexpected reading: %+v", reading)
	}
}
