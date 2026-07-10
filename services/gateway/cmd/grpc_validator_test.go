package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
)

// hangingAuthClient simulates a hung auth service: ValidateToken blocks until
// its context is cancelled (by the bounded-timeout wrap), then returns the gRPC
// DeadlineExceeded status the real client surfaces on a deadline. hardStop lets
// the test drain the goroutine during cleanup if the wrap is ever missing, so a
// reverted wrap fails the test instead of leaking a goroutine forever.
type hangingAuthClient struct {
	hardStop <-chan struct{}
}

func (h *hangingAuthClient) ValidateToken(ctx context.Context, _ *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	case <-h.hardStop:
		return nil, errors.New("test cleanup")
	}
}

// TestGRPCTokenValidator_HangingBackend_ReturnsWithinTimeout is the wrap
// regression guard: with the per-call context.WithTimeout in place, a hung auth
// backend returns within the configured timeout (plus margin) with the gRPC
// DeadlineExceeded status the real client surfaces on a deadline. Reverting the
// wrap passes the unbounded background context to the client, which blocks
// forever, so this test hangs until it hits the deadline below and fails.
func TestGRPCTokenValidator_HangingBackend_ReturnsWithinTimeout(t *testing.T) {
	const timeout = 50 * time.Millisecond

	hardStop := make(chan struct{})
	t.Cleanup(func() { close(hardStop) })

	validator := &GRPCTokenValidator{
		client:  &hangingAuthClient{hardStop: hardStop},
		timeout: timeout,
	}

	done := make(chan error, 1)
	go func() {
		_, err := validator.ValidateToken(context.Background(), "token")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a deadline error from the hung backend, got nil")
		}
		// The real client surfaces a context deadline as a gRPC status error,
		// which does not unwrap to context.DeadlineExceeded; the gRPC-code prong
		// (mirrored in AccessControl) is what classifies it as a timeout. Assert
		// it survives the fmt.Errorf %w wrap in ValidateToken.
		if st, ok := status.FromError(err); !ok || st.Code() != codes.DeadlineExceeded {
			t.Errorf("error does not carry gRPC DeadlineExceeded through the %%w wrap: ok=%v err=%v", ok, err)
		}
	case <-time.After(timeout + 500*time.Millisecond):
		t.Fatal("ValidateToken did not return within the timeout window; the context.WithTimeout bound is missing")
	}
}

// TestGatewayValidateTimeout_EndToEnd_Returns503 wires the production path:
// AccessControl in front of a real GRPCTokenValidator backed by a hung auth
// client. It is the behavioral regression guard for the whole feature: with the
// bound in place, a hung backend makes AccessControl return 503
// SERVICE_UNAVAILABLE within the timeout window (not after client disconnect);
// reverting the context.WithTimeout wrap leaves the client blocked on an
// unbounded context, so ServeHTTP never returns and this test fails at the
// deadline below.
func TestGatewayValidateTimeout_EndToEnd_Returns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const timeout = 50 * time.Millisecond

	hardStop := make(chan struct{})
	t.Cleanup(func() { close(hardStop) })

	validator := &GRPCTokenValidator{
		client:  &hangingAuthClient{hardStop: hardStop},
		timeout: timeout,
	}

	engine := gin.New()
	engine.Use(access.AccessControl(validator, access.GatewayResolve, slog.New(slog.NewTextHandler(io.Discard, nil))))
	engine.GET("/api/finance/periods", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		engine.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503 Service Unavailable", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SERVICE_UNAVAILABLE") {
			t.Errorf("body = %q, want it to contain SERVICE_UNAVAILABLE", rec.Body.String())
		}
	case <-time.After(timeout + 500*time.Millisecond):
		t.Fatal("AccessControl did not return within the timeout window; the ValidateToken bound is missing")
	}
}

// stubAuthClient is a validateTokenClient returning a canned response/error
// without touching gRPC, so the concrete GRPCTokenValidator's field mapping and
// error wrapping are exercised directly. The access package's tests use a
// fakeValidator and never run the real validator, so this closes that gap.
type stubAuthClient struct {
	resp *authpb.ValidateTokenResponse
	err  error
}

func (s *stubAuthClient) ValidateToken(_ context.Context, _ *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	return s.resp, s.err
}

// TestGRPCTokenValidator_MapsResponseFields covers the success-path mapping from
// authpb.ValidateTokenResponse to access.TokenValidationResult, including the
// assumed-session case where AssumedBy is populated.
func TestGRPCTokenValidator_MapsResponseFields(t *testing.T) {
	cases := []struct {
		name string
		resp *authpb.ValidateTokenResponse
		want access.TokenValidationResult
	}{
		{
			name: "plain user session",
			resp: &authpb.ValidateTokenResponse{UserId: "user-1", Role: "user", Username: "alex"},
			want: access.TokenValidationResult{UserID: "user-1", Role: "user", Username: "alex"},
		},
		{
			name: "admin session",
			resp: &authpb.ValidateTokenResponse{UserId: "admin-1", Role: "admin", Username: "root"},
			want: access.TokenValidationResult{UserID: "admin-1", Role: "admin", Username: "root"},
		},
		{
			name: "assumed session carries AssumedBy",
			resp: &authpb.ValidateTokenResponse{UserId: "target-1", Role: "user", Username: "target", AssumedBy: "admin-1"},
			want: access.TokenValidationResult{UserID: "target-1", Role: "user", Username: "target", AssumedBy: "admin-1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := &GRPCTokenValidator{client: &stubAuthClient{resp: tc.resp}, timeout: time.Second}

			got, err := validator.ValidateToken(context.Background(), "token")
			if err != nil {
				t.Fatalf("ValidateToken() error = %v, want nil", err)
			}
			if *got != tc.want {
				t.Errorf("ValidateToken() = %+v, want %+v", *got, tc.want)
			}
		})
	}
}

// TestGRPCTokenValidator_NonDeadlineError_Wraps asserts a non-deadline client
// error is returned wrapped (the fmt.Errorf %w prefix) with its underlying gRPC
// status preserved, so it does not read as a validation timeout on either
// isValidationTimeout prong.
func TestGRPCTokenValidator_NonDeadlineError_Wraps(t *testing.T) {
	clientErr := status.Error(codes.Unauthenticated, "invalid token")
	validator := &GRPCTokenValidator{client: &stubAuthClient{err: clientErr}, timeout: time.Second}

	_, err := validator.ValidateToken(context.Background(), "token")
	if err == nil {
		t.Fatal("ValidateToken() error = nil, want a wrapped client error")
	}
	if !strings.Contains(err.Error(), "auth service validation failed") {
		t.Errorf("error = %q, want it to carry the wrap prefix", err.Error())
	}
	if !errors.Is(err, clientErr) {
		t.Errorf("error = %v, want it to unwrap to the underlying client error", err)
	}
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Errorf("wrapped error must preserve the gRPC Unauthenticated code: ok=%v st=%v", ok, st)
	}
}

// TestGatewayValidateNonDeadline_EndToEnd_Returns401 wires AccessControl in
// front of the real GRPCTokenValidator whose client returns a non-deadline gRPC
// error, proving the concrete validator's %w wrap preserves the code so
// isValidationTimeout classifies it as a genuine rejection (401), not a 503.
func TestGatewayValidateNonDeadline_EndToEnd_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &GRPCTokenValidator{
		client:  &stubAuthClient{err: status.Error(codes.Unauthenticated, "invalid token")},
		timeout: time.Second,
	}

	engine := gin.New()
	engine.Use(access.AccessControl(validator, access.GatewayResolve, slog.New(slog.NewTextHandler(io.Discard, nil))))
	engine.GET("/api/finance/periods", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 Unauthorized", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
		t.Errorf("body = %q, want it to contain UNAUTHORIZED", rec.Body.String())
	}
}
