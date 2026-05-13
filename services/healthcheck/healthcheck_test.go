package healthcheck_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ItsThompson/gofin/services/healthcheck"
)

func TestRun_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Extract port from test server URL (format: http://127.0.0.1:PORT)
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]

	got := healthcheck.Run(port)
	if got != 0 {
		t.Errorf("Run() = %d, want 0 (healthy)", got)
	}
}

func TestRun_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]

	got := healthcheck.Run(port)
	if got != 1 {
		t.Errorf("Run() = %d, want 1 (unhealthy)", got)
	}
}

func TestRun_Unreachable(t *testing.T) {
	// Use a port that nothing listens on.
	got := healthcheck.Run("19999")
	if got != 1 {
		t.Errorf("Run() = %d, want 1 (unreachable)", got)
	}
}

func TestShouldRun_WithFlag(t *testing.T) {
	if !healthcheck.ShouldRun([]string{"/service", "--healthcheck"}) {
		t.Error("ShouldRun() = false, want true")
	}
}

func TestShouldRun_NoArgs(t *testing.T) {
	if healthcheck.ShouldRun([]string{"/service"}) {
		t.Error("ShouldRun() = true, want false")
	}
}

func TestShouldRun_DifferentArg(t *testing.T) {
	if healthcheck.ShouldRun([]string{"/service", "seed-admin"}) {
		t.Error("ShouldRun() = true, want false")
	}
}
