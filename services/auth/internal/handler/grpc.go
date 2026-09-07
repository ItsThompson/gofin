package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/config"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	"github.com/ItsThompson/gofin/services/errkit"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

// GRPCHandler serves the AuthService gRPC surface; Register and Login are
// intentionally Unimplemented (use the REST endpoints).
type GRPCHandler struct {
	pb.UnimplementedAuthServiceServer
	authService *service.AuthService
	logger      *slog.Logger
}

func NewGRPCHandler(authService *service.AuthService, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		authService: authService,
		logger:      logger,
	}
}

// isMissingUser reports whether err is the service's "user not found" signal,
// which GetUserByID surfaces as a 401 *apierr.Error rather than 404.
func isMissingUser(err error) bool {
	var apiErr *apierr.Error
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized
}

// reportServerFailure reports only server (5xx) errors. Client errors are
// already recorded at the guard that produced them; reporting them here would
// bill error quota for ordinary client input.
func reportServerFailure(ctx context.Context, err error, meta errkit.Meta) {
	if apierr.IsServerError(err) {
		_ = errkit.Report(ctx, err, meta)
	}
}

func (h *GRPCHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	result, err := h.authService.ValidateToken(ctx, req.GetAccessToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	return &pb.ValidateTokenResponse{
		UserId:    result.UserID,
		Role:      result.Role,
		Username:  result.Username,
		AssumedBy: result.AssumedBy,
	}, nil
}

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
			Domain: config.ReportDomain,
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
			Domain: config.ReportDomain,
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
			Domain: config.ReportDomain,
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
			Domain: config.ReportDomain,
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
