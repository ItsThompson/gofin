// This file provides the real immudb client for production/Docker builds.
// It requires the github.com/codenotary/immudb dependency, which is fetched
// during `go mod download` in the Docker build stage.
//
// Session management strategy:
// The immudb SDK does not auto-reconnect when a session is invalidated. It
// provides two mechanisms for the application to handle this:
//
//  1. ErrorHandler callback: fires when the heartbeat detects a dead session.
//     We use this for proactive reconnection (recovers within one heartbeat
//     cycle, ~1 minute by default).
//
//  2. Operation-level detection: if a request arrives before the heartbeat
//     fires, the operation itself will fail with "session not found". We
//     detect this and retry once after reconnecting.
//
// Together these ensure the service self-heals from session loss without
// requiring a container restart.

//go:build docker

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codenotary/immudb/pkg/api/schema"
	immudb "github.com/codenotary/immudb/pkg/client"

	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
)

// reconnectReportWindow bounds how often a failed reconnection is reported.
//
// The SDK heartbeat drives this roughly once a minute for as long as immudb is
// unreachable, and a session error makes it reachable per request as well, so a
// day-long outage is on the order of 1,440 failures from one incident. The monthly
// event allowance is 5,000 and it is shared across the organization, so one
// incident would take a fifth of it. One event an hour keeps the same outage near
// two dozen.
const reconnectReportWindow = time.Hour

// realImmudbClient wraps the immudb native Go client with automatic session
// reconnection on session loss.
//
// A generation counter prevents thundering herd reconnections: if the
// heartbeat handler and multiple request goroutines all detect the error
// concurrently, only one performs the reconnect and the others simply retry
// with the fresh session.
type realImmudbClient struct {
	mu         sync.Mutex
	client     immudb.ImmuClient
	generation uint64
	cfg        *config.Config
	logger     *slog.Logger
	// reports bounds the reconnection-failure events. One client is one immudb, so
	// the bound is per store by construction.
	reports *errkit.Limiter
}

func newImmudbClientImpl(ctx context.Context, cfg *config.Config) (repository.ImmudbClient, error) {
	rc := &realImmudbClient{
		cfg:     cfg,
		logger:  slog.Default(),
		reports: errkit.NewLimiter(reconnectReportWindow),
	}

	client, err := rc.openSession(ctx)
	if err != nil {
		return nil, err
	}
	rc.client = client

	return rc, nil
}

// openSession creates a new immudb client with an ErrorHandler that triggers
// proactive reconnection when the heartbeat detects session loss.
func (c *realImmudbClient) openSession(ctx context.Context) (immudb.ImmuClient, error) {
	parts := strings.SplitN(c.cfg.ImmudbAddr, ":", 2)
	host := parts[0]
	port := 3322
	if len(parts) == 2 {
		parsed, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parsing immudb port: %w", err)
		}
		port = parsed
	}

	opts := immudb.DefaultOptions().WithAddress(host).WithPort(port)
	client := immudb.NewClient().WithOptions(opts)

	// Register the SDK's ErrorHandler hook: called by the heartbeater
	// goroutine when keep-alive fails. This triggers proactive reconnection
	// so the session is restored before the next user request arrives.
	client.WithErrorHandler(func(sessionID string, err error) {
		if !isSessionError(err) {
			return
		}
		c.logger.Warn("heartbeat detected session loss, reconnecting proactively...",
			slog.String("session_id", sessionID),
			slog.String("error", err.Error()),
		)
		c.mu.Lock()
		defer c.mu.Unlock()
		_ = c.reconnectLocked(context.Background(), c.generation)
	})

	err := client.OpenSession(ctx, []byte(c.cfg.ImmudbUsername), []byte(c.cfg.ImmudbPassword), "defaultdb")
	if err != nil {
		return nil, fmt.Errorf("opening immudb session: %w", err)
	}

	return client, nil
}

// isSessionError returns true if the error indicates the immudb session
// has been invalidated and a reconnection is needed.
func isSessionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "session not found") ||
		strings.Contains(msg, "no session found") ||
		strings.Contains(msg, "session does not exist")
}

// reconnectLocked closes the dead session (best-effort) and opens a fresh one.
// Only reconnects if the generation hasn't changed since the caller observed
// the error (prevents thundering herd). Caller must hold c.mu.
func (c *realImmudbClient) reconnectLocked(ctx context.Context, observedGen uint64) error {
	// Another goroutine already reconnected: skip.
	if c.generation != observedGen {
		return nil
	}

	c.logger.Warn("immudb session lost, reconnecting...")

	// Best-effort close of the dead session.
	_ = c.client.CloseSession(ctx)

	// Brief pause before reconnecting to avoid hammering immudb.
	time.Sleep(500 * time.Millisecond)

	newClient, err := c.openSession(ctx)
	if err != nil {
		// The record stays per attempt; only the event is bounded. Both the heartbeat
		// and any request that hits a dead session reach this line, so an outage
		// produces one failure a minute at least.
		c.logger.Error("immudb reconnection failed", slog.String("error", err.Error()))
		if c.reports.Allow() {
			// GroupExact, so the whole outage is one issue: the stack differs between the
			// heartbeat goroutine and a request that found a dead session, while the
			// meaning is the same either way.
			_ = errkit.Report(ctx, err, errkit.Meta{
				Kind:       errkit.KindDatabase,
				Op:         "immudb.reconnect",
				Domain:     "expenses",
				Msg:        "immudb reconnection failed",
				GroupKey:   "immudb.unreachable",
				GroupExact: true,
				Data:       map[string]any{"addr": c.cfg.ImmudbAddr},
			})
		}
		return fmt.Errorf("reconnecting to immudb: %w", err)
	}

	c.client = newClient
	c.generation++
	c.logger.Info("immudb session re-established")
	return nil
}

func (c *realImmudbClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*repository.SQLResult, error) {
	c.mu.Lock()
	client := c.client
	gen := c.generation
	c.mu.Unlock()

	_, err := client.SQLExec(ctx, sql, params)
	if err == nil {
		return &repository.SQLResult{}, nil
	}

	if !isSessionError(err) {
		return nil, err
	}

	// Session invalidated: reconnect and retry once.
	c.mu.Lock()
	reconnErr := c.reconnectLocked(ctx, gen)
	client = c.client
	c.mu.Unlock()
	if reconnErr != nil {
		return nil, fmt.Errorf("SQLExec session recovery failed: %w (original: %w)", reconnErr, err)
	}

	_, retryErr := client.SQLExec(ctx, sql, params)
	if retryErr != nil {
		return nil, retryErr
	}
	return &repository.SQLResult{}, nil
}

func (c *realImmudbClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*repository.SQLResult, error) {
	c.mu.Lock()
	client := c.client
	gen := c.generation
	c.mu.Unlock()

	result, err := client.SQLQuery(ctx, sql, params, true)
	if err == nil {
		return toSQLResult(result), nil
	}

	if !isSessionError(err) {
		return nil, err
	}

	// Session invalidated: reconnect and retry once.
	c.mu.Lock()
	reconnErr := c.reconnectLocked(ctx, gen)
	client = c.client
	c.mu.Unlock()
	if reconnErr != nil {
		return nil, fmt.Errorf("SQLQuery session recovery failed: %w (original: %w)", reconnErr, err)
	}

	result, err = client.SQLQuery(ctx, sql, params, true)
	if err != nil {
		return nil, err
	}
	return toSQLResult(result), nil
}

// toSQLResult converts the immudb SDK query result into our repository types.
func toSQLResult(result *schema.SQLQueryResult) *repository.SQLResult {
	sqlRows := make([]repository.SQLRow, len(result.Rows))
	for i, row := range result.Rows {
		values := make([]repository.SQLValue, len(row.Values))
		for j, val := range row.Values {
			values[j] = &immudbValue{val: val}
		}
		sqlRows[i] = repository.SQLRow{Values: values}
	}
	return &repository.SQLResult{Rows: sqlRows}
}

// immudbValue wraps an immudb SQL value to satisfy repository.SQLValue.
type immudbValue struct {
	val *schema.SQLValue
}

func (v *immudbValue) GetString() string { return v.val.GetS() }
func (v *immudbValue) GetInt() int64     { return v.val.GetN() }
func (v *immudbValue) GetBool() bool     { return v.val.GetB() }
