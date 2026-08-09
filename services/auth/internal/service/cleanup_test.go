package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStartPeriodicCleanup_CallsCleanupAfterInterval(t *testing.T) {
	blacklistRepo := new(mockBlacklistRepository)
	repo := new(mockUserRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4)
	svc := NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track calls with a channel
	called := make(chan struct{}, 10)
	blacklistRepo.On("CleanupExpired", mock.Anything).Run(func(args mock.Arguments) {
		called <- struct{}{}
	}).Return(nil)

	svc.StartPeriodicCleanup(ctx, 50*time.Millisecond, 30*time.Second)

	// Wait for at least one cleanup call
	select {
	case <-called:
		// Success: cleanup was called after interval
	case <-time.After(500 * time.Millisecond):
		t.Fatal("CleanupExpired was not called within expected time")
	}

	blacklistRepo.AssertCalled(t, "CleanupExpired", mock.Anything)
}

func TestStartPeriodicCleanup_StopsOnContextCancellation(t *testing.T) {
	blacklistRepo := new(mockBlacklistRepository)
	repo := new(mockUserRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4)
	svc := NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	ctx, cancel := context.WithCancel(context.Background())

	var callCount atomic.Int32
	blacklistRepo.On("CleanupExpired", mock.Anything).Run(func(args mock.Arguments) {
		callCount.Add(1)
	}).Return(nil)

	svc.StartPeriodicCleanup(ctx, 50*time.Millisecond, 30*time.Second)

	// Let a few ticks fire
	time.Sleep(150 * time.Millisecond)

	// Cancel the context to stop the goroutine
	cancel()

	// Wait for any in-flight goroutines to complete after cancellation
	time.Sleep(100 * time.Millisecond)

	// Record the count after cancellation and in-flight work settles
	countAfterCancel := callCount.Load()

	// Wait to confirm no more calls happen
	time.Sleep(200 * time.Millisecond)

	countFinal := callCount.Load()
	assert.Equal(t, countAfterCancel, countFinal,
		"cleanup should not be called after context cancellation")
}

func TestStartPeriodicCleanup_SingleFlight(t *testing.T) {
	blacklistRepo := new(mockBlacklistRepository)
	repo := new(mockUserRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4)
	svc := NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Block the first cleanup so subsequent ticks are skipped
	firstCallStarted := make(chan struct{})
	unblock := make(chan struct{})
	var callCount atomic.Int32

	blacklistRepo.On("CleanupExpired", mock.Anything).Run(func(args mock.Arguments) {
		count := callCount.Add(1)
		if count == 1 {
			close(firstCallStarted)
			<-unblock // Block until we release
		}
	}).Return(nil)

	svc.StartPeriodicCleanup(ctx, 30*time.Millisecond, 5*time.Second)

	// Wait for the first cleanup to start
	select {
	case <-firstCallStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first cleanup did not start")
	}

	// Let several ticks fire while the first cleanup is still blocked
	time.Sleep(150 * time.Millisecond)

	// Only one call should have been made (the rest were skipped)
	assert.Equal(t, int32(1), callCount.Load(),
		"only one cleanup should run concurrently")

	// Unblock the first cleanup
	close(unblock)

	// Now a subsequent tick should fire successfully
	time.Sleep(100 * time.Millisecond)

	assert.Greater(t, callCount.Load(), int32(1),
		"cleanup should resume after the blocked one completes")
}

// TestStartPeriodicCleanup_RecoversAPanickingRepoAndKeepsTicking pins the panic
// guard on the cleanup goroutine. It runs outside any request recovery, so an
// unrecovered repo panic would take the whole auth process down; recovering per
// run means the next tick still fires.
func TestStartPeriodicCleanup_RecoversAPanickingRepoAndKeepsTicking(t *testing.T) {
	blacklistRepo := new(mockBlacklistRepository)
	repo := new(mockUserRepository)
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4)
	svc := NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var callCount atomic.Int32
	blacklistRepo.On("CleanupExpired", mock.Anything).Run(func(mock.Arguments) {
		if callCount.Add(1) == 1 {
			panic("repo exploded")
		}
	}).Return(nil)

	svc.StartPeriodicCleanup(ctx, 30*time.Millisecond, 5*time.Second)

	require.Eventually(t, func() bool {
		return callCount.Load() >= 2
	}, 2*time.Second, 10*time.Millisecond, "the ticker must survive a panicking cleanup run")

	records := errorRecords(t, logs)
	require.Len(t, records, 1, "the recovered panic must produce exactly one record")
	assert.Equal(t, "recovered panic in blacklist cleanup", records[0]["msg"])
	assert.Equal(t, "panic: repo exploded", records[0]["panic"])
	assert.Equal(t, "StartPeriodicCleanup", records[0]["method"])
	assert.Contains(t, records[0]["stack"], "runtime/debug.Stack")
}

// syncBuffer is a mutex-guarded log sink: the cleanup goroutine writes records
// while the test reads them.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}

// errorRecords parses the error-level JSON slog records written to logs.
func errorRecords(t *testing.T, logs *syncBuffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(logs.bytes()))
	for decoder.More() {
		var record map[string]any
		require.NoError(t, decoder.Decode(&record))
		if record["level"] == "ERROR" {
			records = append(records, record)
		}
	}
	return records
}

func TestStartPeriodicCleanup_LogsErrorOnFailure(t *testing.T) {
	blacklistRepo := new(mockBlacklistRepository)
	repo := new(mockUserRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4)
	svc := NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := make(chan struct{}, 10)
	blacklistRepo.On("CleanupExpired", mock.Anything).Run(func(args mock.Arguments) {
		called <- struct{}{}
	}).Return(assert.AnError)

	svc.StartPeriodicCleanup(ctx, 50*time.Millisecond, 30*time.Second)

	// Wait for cleanup to be called (it will fail but the goroutine continues)
	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("CleanupExpired was not called")
	}

	// Wait for a second call to confirm the goroutine keeps running after errors
	select {
	case <-called:
		// Success: goroutine continued after error
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cleanup goroutine stopped after error")
	}
}
