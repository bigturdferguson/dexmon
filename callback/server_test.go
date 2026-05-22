package callback_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dexmon/callback"
	"dexmon/store"
	"dexmon/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCallback_ClearsReceiptOnAck(t *testing.T) {
	st := newTestStore(t)
	rid := "receipt-123"
	expires := time.Now().UTC().Add(2 * time.Hour)
	now := time.Now().UTC()
	_ = st.UpsertAlarmState(types.AlarmState{
		Account:          "jessica",
		AlarmName:        "Severe Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	})

	srv := callback.New(st, 0)
	body, _ := json.Marshal(map[string]interface{}{
		"receipt":         "receipt-123",
		"acknowledged_at": time.Now().Unix(),
		"snooze":          0,
	})
	req := httptest.NewRequest(http.MethodPost, "/pushover/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	state, _ := st.GetAlarmState("jessica", "Severe Low", "brandon")
	if state.ReceiptID != nil {
		t.Error("expected ReceiptID cleared on acknowledgment")
	}
}

func TestCallback_SetsSnoozedUntilWhenSnoozeProvided(t *testing.T) {
	st := newTestStore(t)
	rid := "receipt-456"
	expires := time.Now().UTC().Add(2 * time.Hour)
	now := time.Now().UTC()
	_ = st.UpsertAlarmState(types.AlarmState{
		Account:          "jessica",
		AlarmName:        "Low",
		Recipient:        "brandon",
		LastFiredAt:      &now,
		ReceiptID:        &rid,
		ReceiptExpiresAt: &expires,
	})

	srv := callback.New(st, 0)
	body, _ := json.Marshal(map[string]interface{}{
		"receipt":         "receipt-456",
		"acknowledged_at": time.Now().Unix(),
		"snooze":          1800, // 30 minutes in seconds
	})
	req := httptest.NewRequest(http.MethodPost, "/pushover/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	state, _ := st.GetAlarmState("jessica", "Low", "brandon")
	if state.ReceiptID != nil {
		t.Error("expected ReceiptID cleared")
	}
	if state.SnoozedUntil == nil {
		t.Fatal("expected SnoozedUntil set")
	}
	minExpected := time.Now().UTC().Add(29 * time.Minute)
	if state.SnoozedUntil.Before(minExpected) {
		t.Errorf("SnoozedUntil %v is earlier than expected ~30m from now", *state.SnoozedUntil)
	}
}

func TestCallback_IgnoresUnknownReceipt(t *testing.T) {
	st := newTestStore(t)
	srv := callback.New(st, 0)

	body, _ := json.Marshal(map[string]interface{}{
		"receipt":         "unknown-receipt",
		"acknowledged_at": time.Now().Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/pushover/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown receipt, got %d", w.Code)
	}
}
