package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

const repoFailure = "connection refused"

// newReportingGRPCHandler builds a gRPC handler whose records land in the
// returned buffer. The buffer is also installed as slog.Default because errkit
// writes its report record through the package-level logger.
func newReportingGRPCHandler(t *testing.T) (*GRPCHandler, *mockUserRepository, *bytes.Buffer) {
	t.Helper()

	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)
	return NewGRPCHandler(authSvc, logger), repo, buf
}

// This is the representative for every internal exit in the handler: an
// unclassified repo failure produces one handler-owned event naming the
// operation, while the wire response stays exactly what it was.
func TestGRPC_InternalFailure_YieldsOneEventNamingTheOperation(t *testing.T) {
	handler, repo, _ := newReportingGRPCHandler(t)

	repo.On("GetUserByID", mock.Anything, "user-1").Return(nil, errors.New(repoFailure))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetUser(ctx, &pb.GetUserRequest{UserId: "user-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "failed to retrieve user", st.Message(), "the wire message does not move")

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "auth.get_user", events[0].Tags["operation"])
	assert.Equal(t, "auth", events[0].Tags["domain"])
	assert.Equal(t, "internal", events[0].Tags["error_kind"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, repoFailure)
	assert.Equal(t, map[string]any{
		"method":  "GetUser",
		"user_id": "user-1",
	}, events[0].Contexts["gofin"])
}

// A missing user is an expected client outcome mapped to codes.NotFound, and it
// must cost no quota: the classification branch sits above the report path.
func TestGRPC_MissingUser_YieldsNoEvent(t *testing.T) {
	handler, repo, _ := newReportingGRPCHandler(t)

	repo.On("GetUserByID", mock.Anything, "user-1").
		Return(nil, apierr.Unauthorized("User not found"))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetUser(ctx, &pb.GetUserRequest{UserId: "user-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Empty(t, transport.Events())
}

// An invalid or expired token is ordinary client input. The 401 is the record:
// no event, and since the warn removal, no log record either.
func TestGRPC_ValidateToken_Rejection_LeavesNoEventAndNoWarnRecord(t *testing.T) {
	handler, _, logs := newReportingGRPCHandler(t)

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.ValidateToken(ctx, &pb.ValidateTokenRequest{AccessToken: "not-a-jwt"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())

	assert.Empty(t, transport.Events())
	assert.Empty(t, warnRecords(t, logs))
}

// warnRecords parses the buffered log output and returns the warn-level
// records only.
func warnRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["level"] == slog.LevelWarn.String() {
			records = append(records, record)
		}
	}
	return records
}

// The logout blacklist failure is fire-and-forget: the caller still receives
// 204, so this handler is the failure's only reporter.
func TestREST_LogoutBlacklistFailure_ReportsOneEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)

	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)
	handler := NewRESTHandler(authSvc, logger, false, "", service.DefaultAccessTokenTTL, service.DefaultRefreshTokenTTL)

	r := gin.New()
	handler.RegisterRoutes(r)

	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-1", "user", "testuser")
	require.NoError(t, err)
	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("BlacklistToken", mock.Anything, refreshClaims.ID, "user-1", mock.AnythingOfType("time.Time")).
		Return(errors.New("blacklist write: connection refused"))

	transport := &errkittest.Transport{}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_refresh", Value: refreshToken})
	req = req.WithContext(errkittest.ContextWithHub(req.Context(), transport))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code, "the caller still receives 204")

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "auth.logout", events[0].Tags["operation"])
	assert.Equal(t, "auth", events[0].Tags["domain"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, "connection refused")
}
