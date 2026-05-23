package health_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dexmon/health"
)

func TestPingWatchdog_LogsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	}()

	health.PingWatchdog(srv.URL)

	if !strings.Contains(buf.String(), "watchdog ping ok") {
		t.Errorf("expected 'watchdog ping ok' in log output, got: %q", buf.String())
	}
}
