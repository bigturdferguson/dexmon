package callback

import (
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
	store *store.Store
	port  int
	mux   *http.ServeMux
}

func New(st *store.Store, port int, account string, alarms []config.AlarmConfig, recipients map[string]config.RecipientConfig) *Server {
	s := &Server{store: st, port: port, mux: http.NewServeMux()}
	dash := dashboard.New(st, account, alarms, recipients)
	s.mux.Handle("GET /", dash)
	s.mux.Handle("GET /api/dashboard", dash)
	s.mux.HandleFunc("POST /pushover/callback", s.handleCallback)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("callback server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

type callbackPayload struct {
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
