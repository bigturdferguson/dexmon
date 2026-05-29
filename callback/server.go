package callback

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"dexmon/config"
	"dexmon/dashboard"
	"dexmon/store"
)

type Server struct {
	store    *store.Store
	port     int
	mux      *http.ServeMux
	appToken string
}

func New(st *store.Store, port int, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig, targetLow, targetHigh int, watchdogURL string, appToken string) *Server {
	s := &Server{store: st, port: port, mux: http.NewServeMux(), appToken: appToken}
	dash := dashboard.New(st, account, alarms, recipients, targetLow, targetHigh, watchdogURL)
	s.mux.Handle("GET /", dash)
	s.mux.Handle("GET /api/dashboard", dash)
	s.mux.HandleFunc("POST /pushover/callback", s.handleCallback)
	return s
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	securityHeaders(s.mux).ServeHTTP(w, r)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("callback server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

type callbackPayload struct {
	Token          string `json:"token"`
	Receipt        string `json:"receipt"`
	AcknowledgedAt int64  `json:"acknowledged_at"`
	Snooze         int    `json:"snooze"`
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	var payload callbackPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if s.appToken != "" && subtle.ConstantTimeCompare([]byte(payload.Token), []byte(s.appToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	state, err := s.store.GetAlarmStateByReceiptID(payload.Receipt)
	if err != nil {
		log.Printf("callback: lookup receipt %s: %v", payload.Receipt, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if state == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	state.ReceiptID = nil
	state.ReceiptExpiresAt = nil

	if payload.Snooze > 0 {
		snoozedUntil := time.Now().UTC().Add(time.Duration(payload.Snooze) * time.Second)
		state.SnoozedUntil = &snoozedUntil
	} else {
		state.SnoozedUntil = nil
	}

	if err := s.store.UpsertAlarmState(*state); err != nil {
		log.Printf("callback: update state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if payload.Snooze > 0 {
		snooze := time.Duration(payload.Snooze) * time.Second
		log.Printf("callback: %s/%q/%s acknowledged, snoozed %s", state.Account, state.AlarmName, state.Recipient, snooze)
	} else {
		log.Printf("callback: %s/%q/%s acknowledged", state.Account, state.AlarmName, state.Recipient)
	}
	w.WriteHeader(http.StatusOK)
}
