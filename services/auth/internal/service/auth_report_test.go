package service

import (
	"context"
	"errors"
	"testing"

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
