package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

// ValidateToken masks a revocation-check DB failure behind a 401, so the
// service is the failure's only reporter: the report carries the raw DB error
// while the caller still receives the unchanged 401.
func TestValidateToken_RevocationCheckFailure_IsReportedNotLeaked(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)

	jwtSvc := NewJWTService("test-secret")
	access, _, err := jwtSvc.GenerateTokenPair("user-1", "user", "johndoe")
	require.NoError(t, err)

	dbFailure := errors.New("tokens_revoked_at lookup: connection refused")
	repo.On("GetTokensRevokedAt", mock.Anything, "user-1").Return(nil, dbFailure)

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err = svc.ValidateToken(ctx, access)
	require.Error(t, err)

	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.Status)
	assert.Equal(t, "Unable to validate token", apiErr.Message, "the wire response does not move")

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "auth.validate_token", events[0].Tags["operation"])
	assert.Equal(t, "auth", events[0].Tags["domain"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value,
		"connection refused", "the report carries the raw DB error")
	assert.Equal(t, map[string]any{"user_id": "user-1"}, events[0].Contexts["gofin"])
}

// bindRecordingHub binds a client recording into the returned transport to the
// process-wide hub, and restores the previous client afterwards. The cleanup
// runs in a background goroutine with no request hub, so its report reaches
// Sentry through the hub-clone fallback of sentry.CurrentHub().
//
// Tests using it must not run in parallel.
func bindRecordingHub(t *testing.T) *errkittest.Transport {
	t.Helper()

	transport := &errkittest.Transport{}
	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
	sentry.CurrentHub().BindClient(errkittest.NewClient(transport))

	return transport
}

// The periodic cleanup failure is fire-and-forget: the ticker keeps running, so
// this service is the failure's only reporter. The report reaches Sentry through
// the background-hub fallback.
func TestStartPeriodicCleanup_CleanupFailure_ReportsOneEvent(t *testing.T) {
	blacklistRepo := new(mockBlacklistRepository)
	repo := new(mockUserRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)

	called := make(chan struct{}, 10)
	blacklistRepo.On("CleanupExpired", mock.Anything).Run(func(args mock.Arguments) {
		called <- struct{}{}
	}).Return(errors.New("cleanup: connection refused"))

	transport := bindRecordingHub(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartPeriodicCleanup(ctx, 50*time.Millisecond, 30*time.Second)

	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("CleanupExpired was not called")
	}

	// Give the report a moment to flush through the hub before asserting.
	require.Eventually(t, func() bool {
		return len(transport.Events()) >= 1
	}, 500*time.Millisecond, 10*time.Millisecond)

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "auth.blacklist_cleanup", events[0].Tags["operation"])
	assert.Equal(t, "auth", events[0].Tags["domain"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, "connection refused")
}
