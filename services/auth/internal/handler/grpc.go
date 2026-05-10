package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/auth/internal/service"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

// GRPCHandler implements the AuthService gRPC server.
// Register, Login, and ValidateToken are implemented.
// All other RPCs return Unimplemented (stubs for later tickets).
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

func (h *GRPCHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	result, err := h.authService.ValidateToken(ctx, req.GetAccessToken())
	if err != nil {
		h.logger.Warn("token validation failed",
			slog.String("method", "ValidateToken"),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	return &pb.ValidateTokenResponse{
		UserId:    result.UserID,
		Role:      result.Role,
		Username:  result.Username,
		AssumedBy: result.AssumedBy,
	}, nil
}

// Register is exposed via gRPC for the gateway. The REST handler is the primary
// consumer, but the gateway may call via gRPC in the future.
func (h *GRPCHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "use REST endpoint POST /api/auth/register")
}

func (h *GRPCHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "use REST endpoint POST /api/auth/login")
}

func (h *GRPCHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RefreshToken not yet implemented")
}

func (h *GRPCHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Logout not yet implemented")
}

func (h *GRPCHandler) AssumeIdentity(ctx context.Context, req *pb.AssumeIdentityRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "AssumeIdentity not yet implemented")
}

func (h *GRPCHandler) RestoreIdentity(ctx context.Context, req *pb.RestoreIdentityRequest) (*pb.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RestoreIdentity not yet implemented")
}

func (h *GRPCHandler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListUsers not yet implemented")
}

func (h *GRPCHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		if authErr, ok := err.(*service.AuthError); ok && authErr.Status == 401 {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		h.logger.Error("failed to get user",
			slog.String("method", "GetUser"),
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
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

func (h *GRPCHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UpdateUser not yet implemented")
}

func (h *GRPCHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ChangePassword not yet implemented")
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
		if authErr, ok := err.(*service.AuthError); ok && authErr.Status == 401 {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		h.logger.Error("failed to look up user for password verification",
			slog.String("method", "VerifyPassword"),
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
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
		h.logger.Error("failed to delete refresh token blacklist",
			slog.String("method", "DeleteUserData"),
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to delete refresh tokens")
	}

	// Delete the user row
	if err := h.authService.DeleteUserRow(ctx, userID); err != nil {
		h.logger.Error("failed to delete user row",
			slog.String("method", "DeleteUserData"),
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to delete user")
	}

	h.logger.Info("user data deleted via gRPC",
		slog.String("method", "DeleteUserData"),
		slog.String("user_id", userID),
	)

	return &pb.DeleteUserDataResponse{}, nil
}
