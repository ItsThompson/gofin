package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thompsnt/gofin/services/auth/internal/db"
	"github.com/thompsnt/gofin/services/auth/internal/model"
)

// PostgresUserRepository implements UserRepository using sqlc-generated queries.
type PostgresUserRepository struct {
	queries *db.Queries
}

// NewPostgresUserRepository creates a new PostgresUserRepository.
func NewPostgresUserRepository(queries *db.Queries) *PostgresUserRepository {
	return &PostgresUserRepository{queries: queries}
}

func (r *PostgresUserRepository) CreateUser(ctx context.Context, username, email, passwordHash, role, currency string) (*model.User, error) {
	dbUser, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Currency:     currency,
	})
	if err != nil {
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	dbUser, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}

	dbUser, err := r.queries.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	dbUser, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

// dbUserToModel converts a sqlc-generated AuthUser to the domain model.
func dbUserToModel(u db.AuthUser) *model.User {
	id := ""
	if u.ID.Valid {
		idBytes := u.ID.Bytes
		id = formatUUID(idBytes)
	}

	return &model.User{
		ID:                     id,
		Username:               u.Username,
		Email:                  u.Email,
		PasswordHash:           u.PasswordHash,
		Role:                   u.Role,
		Currency:               u.Currency,
		HasCompletedOnboarding: u.HasCompletedOnboarding,
		CreatedAt:              u.CreatedAt.Time,
		UpdatedAt:              u.UpdatedAt.Time,
	}
}

// formatUUID formats a [16]byte as a UUID string.
func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
