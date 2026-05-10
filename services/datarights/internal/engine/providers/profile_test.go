package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

// mockAuthServiceClient implements the subset of AuthServiceClient needed for tests.
type mockAuthServiceClient struct {
	getUserResp *authpb.UserResponse
	getUserErr  error
}

func (m *mockAuthServiceClient) GetUser(_ context.Context, req *authpb.GetUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	if m.getUserErr != nil {
		return nil, m.getUserErr
	}
	return m.getUserResp, nil
}

// Implement remaining interface methods as no-ops for the mock.
func (m *mockAuthServiceClient) Register(_ context.Context, _ *authpb.RegisterRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Login(_ context.Context, _ *authpb.LoginRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ValidateToken(_ context.Context, _ *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) RefreshToken(_ context.Context, _ *authpb.RefreshTokenRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Logout(_ context.Context, _ *authpb.LogoutRequest, _ ...grpc.CallOption) (*authpb.LogoutResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) AssumeIdentity(_ context.Context, _ *authpb.AssumeIdentityRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) RestoreIdentity(_ context.Context, _ *authpb.RestoreIdentityRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ListUsers(_ context.Context, _ *authpb.ListUsersRequest, _ ...grpc.CallOption) (*authpb.ListUsersResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) UpdateUser(_ context.Context, _ *authpb.UpdateUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ChangePassword(_ context.Context, _ *authpb.ChangePasswordRequest, _ ...grpc.CallOption) (*authpb.ChangePasswordResponse, error) {
	return nil, nil
}

func TestProfileProvider_Name(t *testing.T) {
	p := NewProfileProvider(nil)
	assert.Equal(t, "profile", p.Name())
}

func TestProfileProvider_Headers(t *testing.T) {
	p := NewProfileProvider(nil)
	expected := []string{"username", "email", "currency", "role", "account_created_at"}
	assert.Equal(t, expected, p.Headers())
}

func TestProfileProvider_Collect_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		getUserResp: &authpb.UserResponse{
			Id:        "user-123",
			Username:  "alex",
			Email:     "alex@example.com",
			Currency:  "USD",
			Role:      "user",
			CreatedAt: "2025-03-15T10:30:00Z",
		},
	}

	p := NewProfileProvider(mockClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)

	expected := []string{"alex", "alex@example.com", "USD", "user", "2025-03-15T10:30:00Z"}
	assert.Equal(t, expected, rows[0])
}

func TestProfileProvider_Collect_AdminRole(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		getUserResp: &authpb.UserResponse{
			Id:        "admin-1",
			Username:  "admin",
			Email:     "admin@example.com",
			Currency:  "EUR",
			Role:      "admin",
			CreatedAt: "2024-01-01T00:00:00Z",
		},
	}

	p := NewProfileProvider(mockClient)
	rows, err := p.Collect(context.Background(), "admin-1")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "admin", rows[0][3])
}

func TestProfileProvider_Collect_Error(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		getUserErr: fmt.Errorf("connection refused"),
	}

	p := NewProfileProvider(mockClient)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching user profile")
}
