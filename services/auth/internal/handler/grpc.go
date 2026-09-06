package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	"github.com/ItsThompson/gofin/services/errkit"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

// reportDomain is the domain tag on every report this service makes, shared
// with the other services' tag vocabulary so one cross-project query covers it.
const reportDomain = "auth"

// GRPCHandler implements the AuthService gRPC server. Register and Login
// deliberately return codes.Unimplemented directing callers to the REST
// endpoints. RPCs without an explicit method are served by the embedded
// UnimplementedAuthServiceServer.
type GRPCHandler struct {
	pb.UnimplementedAuthServiceServer
	authService *service.AuthService
	logger      *slog.Logger
}

// NewGRPCHandler creates a new GRPCHandler.
func NewGRPCHandler(authService *service.AuthService, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		authService: authService,
		logger:      logger,
	}
}

// isMissingUser reports whether err is (or wraps) the service's "user not
// found" signal, which GetUserByID surfaces as a 401 *apierr.Error. errors.As
// unwraps %w chains, so a wrapped typed error still classifies correctly (C7).
func isMissingUser(err error) bool {
	var apiErr *apierr.Error
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized
}

// reportServerFailure reports err unless the client is about to receive a client
// error. Every codes.Internal exit below is reachable with a typed *apierr.Error
// whose code the exit does not name, and a code this handler does not map is not
// evidence that the failure is the service's fault: a validation or not-found error
// arriving at one of them would otherwise bill error quota for ordinary client
// input, and nothing would fail.
//
// The gate is the rendered status rather than a list of codes, so it stays correct
// as the code set grows. A gated error leaves no record here, which is right: the
// guard that produced the typed 4xx recorded it where the decision was made.
func reportServerFailure(ctx context.Context, err error, meta errkit.Meta) {
	if apierr.IsServerError(err) {
		_ = errkit.Report(ctx, err, meta)
	}
}

func (h *GRPCHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	result, err := h.authService.ValidateToken(ctx, req.GetAccessToken())
	if err != nil {
		// An expired or invalid token is ordinary client input: the 401 wire
		// response is the record, and reporting it would bill quota per request.
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	return &pb.ValidateTokenResponse{
		UserId:    result.UserID,
		Role:      result.Role,
		Username:  result.Username,
		AssumedBy: result.AssumedBy,
	}, nil
}

// Register returns codes.Unimplemented; the REST endpoint POST /api/auth/register
// is the sole implementation. The RPC exists to satisfy the generated interface.
func (h *GRPCHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "use REST endpoint POST /api/auth/register")
}

func (h *GRPCHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "use REST endpoint POST /api/auth/login")
}

func (h *GRPCHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		if isMissingUser(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "auth.get_user",
			Domain: reportDomain,
			Msg:    "failed to get user",
			Data: map[string]any{
				"method":  "GetUser",
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to retrieve user")
	}

	return &pb.UserResponse{
		Id:                     user.ID,
		Username:               user.Username,
		Email:                  user.Email,
		Role:                   user.Role,
		Currency:               user.Currency,
		HasCompletedOnboarding: user.HasCompletedOnboarding,
		CreatedAt:              user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (h *GRPCHandler) VerifyPassword(ctx context.Context, req *pb.VerifyPasswordRequest) (*pb.VerifyPasswordResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	password := req.GetPassword()
	if password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		if isMissingUser(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "auth.verify_password",
			Domain: reportDomain,
			Msg:    "failed to look up user",
			Data: map[string]any{
				"method":  "VerifyPassword",
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to look up user")
	}

	valid := h.authService.CheckPassword(password, user.PasswordHash)
	return &pb.VerifyPasswordResponse{Valid: valid}, nil
}

func (h *GRPCHandler) DeleteUserData(ctx context.Context, req *pb.DeleteUserDataRequest) (*pb.DeleteUserDataResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Delete refresh token blacklist entries first (FK constraint)
	if err := h.authService.DeleteRefreshTokenBlacklist(ctx, userID); err != nil {
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "auth.delete_user_data",
			Domain: reportDomain,
			Msg:    "failed to delete refresh tokens",
			Data: map[string]any{
				"method":  "DeleteUserData",
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to delete refresh tokens")
	}

	if err := h.authService.DeleteUserRow(ctx, userID); err != nil {
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "auth.delete_user_data",
			Domain: reportDomain,
			Msg:    "failed to delete user",
			Data: map[string]any{
				"method":  "DeleteUserData",
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to delete user")
	}

	h.logger.Info("user data deleted via gRPC",
		slog.String("method", "DeleteUserData"),
		slog.String("user_id", userID),
	)

	return &pb.DeleteUserDataResponse{}, nil
}
