package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/thompsnt/gofin/services/auth/internal/service"
	pb "github.com/thompsnt/gofin/services/auth/proto/authpb"
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

func (h *GRPCHandler) ValidateToken(_ context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	result, err := h.authService.ValidateToken(req.GetAccessToken())
	if err != nil {
		h.logger.Warn("token validation failed",
			slog.String("service", "auth"),
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
	return nil, status.Error(codes.Unimplemented, "GetUser not yet implemented")
}

func (h *GRPCHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UpdateUser not yet implemented")
}

func (h *GRPCHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ChangePassword not yet implemented")
}
