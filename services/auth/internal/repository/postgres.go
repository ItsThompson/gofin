package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/pgutil"
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
		if constraint, ok := pgutil.IsUniqueViolation(err); ok {
			return nil, &DuplicateError{Constraint: constraint}
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	dbUser, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	uid, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, err
	}

	dbUser, err := r.queries.GetUserByID(ctx, uid)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	dbUser, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if pgutil.IsNoRows(err) {
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
		id = uuid.UUID(u.ID.Bytes).String()
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

func (r *PostgresUserRepository) CompleteOnboarding(ctx context.Context, userID string, currency string) (*model.User, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	dbUser, err := r.queries.CompleteOnboarding(ctx, db.CompleteOnboardingParams{
		Currency: currency,
		ID:       uid,
	})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) ListAllUsers(ctx context.Context) ([]*model.User, error) {
	rows, err := r.queries.ListAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	users := make([]*model.User, 0, len(rows))
	for _, row := range rows {
		id := ""
		if row.ID.Valid {
			id = uuid.UUID(row.ID.Bytes).String()
		}
		users = append(users, &model.User{
			ID:        id,
			Username:  row.Username,
			Email:     row.Email,
			Role:      row.Role,
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return users, nil
}

func (r *PostgresUserRepository) UpdateUser(ctx context.Context, userID, username, email, currency string) (*model.User, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	dbUser, err := r.queries.UpdateUser(ctx, db.UpdateUserParams{
		Username: username,
		Email:    email,
		Currency: currency,
		ID:       uid,
	})
	if err != nil {
		if constraint, ok := pgutil.IsUniqueViolation(err); ok {
			return nil, &DuplicateError{Constraint: constraint}
		}
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbUserToModel(dbUser), nil
}

func (r *PostgresUserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}

	return r.queries.UpdatePassword(ctx, db.UpdatePasswordParams{
		PasswordHash: passwordHash,
		ID:           uid,
	})
}

func (r *PostgresUserRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}

	return r.queries.RevokeAllUserTokens(ctx, uid)
}

func (r *PostgresUserRepository) GetTokensRevokedAt(ctx context.Context, userID string) (*time.Time, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	ts, err := r.queries.GetTokensRevokedAt(ctx, uid)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if !ts.Valid {
		return nil, nil
	}
	t := ts.Time
	return &t, nil
}

func (r *PostgresUserRepository) DeleteUser(ctx context.Context, userID string) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}

	return r.queries.DeleteUser(ctx, uid)
}
