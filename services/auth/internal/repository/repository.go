package repository

import (
	"context"

	"github.com/ItsThompson/gofin/services/auth/internal/model"
)

// UserRepository defines the data access contract for user operations.
// Implementations can be backed by PostgreSQL (production) or mocks (tests).
type UserRepository interface {
	CreateUser(ctx context.Context, username, email, passwordHash, role, currency string) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	CompleteOnboarding(ctx context.Context, userID string, currency string) (*model.User, error)
	ListAllUsers(ctx context.Context) ([]*model.User, error)
}
