package main

import (
	"net"
	"testing"
	"time"
)

// TestRun_RESTBindFailure_ReturnsError pins that when the REST listener cannot
// bind (its port is already in use), run() returns a non-nil error so main()
// exits non-zero and the container restarts, instead of lingering with no listener.
func TestRun_RESTBindFailure_ReturnsError(t *testing.T) {
	// Occupy the wildcard port the gateway will try to bind (":PORT"). Binding
	// the same wildcard address is what makes the conflict deterministic: a
	// 127.0.0.1-only occupant would not reliably collide with the server's
	// ":PORT" bind.
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to occupy a port: %v", err)
	}
	defer func() { _ = occupied.Close() }()

	_, port, err := net.SplitHostPort(occupied.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse occupied port: %v", err)
	}

	t.Setenv("AUTH_SERVICE_ADDR", "auth-service:9081")
	t.Setenv("AUTH_SERVICE_REST", "http://auth-service:8081")
	t.Setenv("EXPENSE_SERVICE_REST", "http://expense-service:8082")
	t.Setenv("FINANCE_SERVICE_REST", "http://finance-service:8083")
	t.Setenv("DATARIGHTS_SERVICE_REST", "http://datarights-service:8084")
	t.Setenv("PORT", port)

	// run() installs its own signal context and, with no signal sent, only
	// returns on a fatal serve error here. Guard against a hang: a regression
	// that reintroduces the zombie bug (swallowing the bind error) would block.
	done := make(chan error, 1)
	go func() { done <- run() }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("run() = nil, want a non-nil error when the REST port cannot bind")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run() did not return within 10s; the REST bind error was swallowed (zombie bug)")
	}
}
