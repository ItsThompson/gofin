package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
)

// mockRepo implements repository.FinanceRepository for service tests.
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) UpsertDefaults(ctx context.Context, settings *model.DefaultSettings) (*model.DefaultSettings, error) {
	args := m.Called(ctx, settings)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DefaultSettings), args.Error(1)
}

func (m *mockRepo) GetDefaults(ctx context.Context, userID string) (*model.DefaultSettings, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DefaultSettings), args.Error(1)
}

func (m *mockRepo) CreateTag(ctx context.Context, userID, name string, isDefault bool) (*model.Tag, error) {
	args := m.Called(ctx, userID, name, isDefault)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Tag), args.Error(1)
}

func (m *mockRepo) GetTag(ctx context.Context, tagID, userID string) (*model.Tag, error) {
	args := m.Called(ctx, tagID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Tag), args.Error(1)
}

func (m *mockRepo) ListTags(ctx context.Context, userID string) ([]*model.Tag, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Tag), args.Error(1)
}

func (m *mockRepo) UpdateTag(ctx context.Context, tagID, userID, name string) (*model.Tag, error) {
	args := m.Called(ctx, tagID, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Tag), args.Error(1)
}

func (m *mockRepo) DeleteTag(ctx context.Context, tagID, userID string) error {
	args := m.Called(ctx, tagID, userID)
	return args.Error(0)
}

func (m *mockRepo) CountUserTags(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepo) CountTagInProRata(ctx context.Context, tagID, userID string) (int64, error) {
	args := m.Called(ctx, tagID, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepo) GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

func (m *mockRepo) CreatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

func (m *mockRepo) ListPeriods(ctx context.Context, userID string) ([]*model.BudgetPeriod, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.BudgetPeriod), args.Error(1)
}

// mockTxBeg implements repository.TxBeginner.
type mockTxBeg struct {
	mock.Mock
}

func (m *mockTxBeg) BeginTx(ctx context.Context) (repository.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repository.Tx), args.Error(1)
}

// mockTxn implements repository.Tx.
type mockTxn struct {
	mock.Mock
	repo repository.FinanceRepository
}

func (m *mockTxn) Commit(ctx context.Context) error   { return m.Called(ctx).Error(0) }
func (m *mockTxn) Rollback(ctx context.Context) error  { return m.Called(ctx).Error(0) }
func (m *mockTxn) Repo() repository.FinanceRepository  { return m.repo }

// mockExpClient implements ExpenseClient for service tests.
type mockExpClient struct {
	mock.Mock
}

func (m *mockExpClient) GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ExpenseData), args.Error(1)
}

func (m *mockExpClient) CountExpensesByTag(ctx context.Context, userID, tagID string) (int64, error) {
	args := m.Called(ctx, userID, tagID)
	return args.Get(0).(int64), args.Error(1)
}

func newTagTestService(repo *mockRepo, txBeg *mockTxBeg, expClient *mockExpClient) *FinanceService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewFinanceService(repo, txBeg, logger)
	if expClient != nil {
		svc.WithExpenseClient(expClient)
	}
	return svc
}

func makeTag(id, name string, isDefault bool) *model.Tag {
	return &model.Tag{
		ID:        id,
		UserID:    "user-1",
		Name:      name,
		IsDefault: isDefault,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// --- ListTags Tests ---

func TestListTags_ReturnsExistingTags(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	tags := []*model.Tag{makeTag("t1", "Bills", true), makeTag("t2", "Food", true)}
	repo.On("CountUserTags", mock.Anything, "user-1").Return(int64(2), nil)
	repo.On("ListTags", mock.Anything, "user-1").Return(tags, nil)

	result, err := svc.ListTags(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Bills", result[0].Name)
}

func TestListTags_LazySeedsWhenNoTags(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	txRepo := new(mockRepo)
	tx := &mockTxn{repo: txRepo}

	svc := newTagTestService(repo, txBeg, nil)

	// User has 0 tags: triggers lazy seeding
	repo.On("CountUserTags", mock.Anything, "user-1").Return(int64(0), nil)
	txBeg.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)

	// Expect all 10 default tags to be created
	for _, tagName := range DefaultTags {
		txRepo.On("CreateTag", mock.Anything, "user-1", tagName, true).
			Return(makeTag("tag-"+tagName, tagName, true), nil)
	}

	// After seeding, list tags returns the seeded tags
	seededTags := make([]*model.Tag, len(DefaultTags))
	for i, tagName := range DefaultTags {
		seededTags[i] = makeTag("tag-"+tagName, tagName, true)
	}
	repo.On("ListTags", mock.Anything, "user-1").Return(seededTags, nil)

	result, err := svc.ListTags(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, result, 10)
	tx.AssertCalled(t, "Commit", mock.Anything)
}

// --- CreateTag Tests ---

func TestCreateTag_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	repo.On("CreateTag", mock.Anything, "user-1", "Groceries", false).
		Return(makeTag("new-tag", "Groceries", false), nil)

	tag, err := svc.CreateTag(context.Background(), "user-1", &model.CreateTagRequest{Name: "Groceries"})
	require.NoError(t, err)
	assert.Equal(t, "Groceries", tag.Name)
	assert.False(t, tag.IsDefault)
}

func TestCreateTag_DuplicateName(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	// Simulate PostgreSQL unique constraint violation
	pgErr := &pgconn.PgError{Code: "23505"}
	repo.On("CreateTag", mock.Anything, "user-1", "Bills", false).
		Return(nil, pgErr)

	tag, err := svc.CreateTag(context.Background(), "user-1", &model.CreateTagRequest{Name: "Bills"})
	assert.Nil(t, tag)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrDuplicateTag, svcErr.Code)
	assert.Equal(t, 409, svcErr.Status)
}

func TestCreateTag_NameTooLong(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	longName := "This tag name is way too long and exceeds the fifty character limit here"
	tag, err := svc.CreateTag(context.Background(), "user-1", &model.CreateTagRequest{Name: longName})
	assert.Nil(t, tag)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
	assert.Contains(t, svcErr.Message, "50 characters")
}

func TestCreateTag_EmptyName(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	tag, err := svc.CreateTag(context.Background(), "user-1", &model.CreateTagRequest{Name: "  "})
	assert.Nil(t, tag)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

// --- UpdateTag Tests ---

func TestUpdateTag_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	repo.On("UpdateTag", mock.Anything, "tag-1", "user-1", "Renamed").
		Return(makeTag("tag-1", "Renamed", true), nil)

	tag, err := svc.UpdateTag(context.Background(), "user-1", "tag-1", &model.UpdateTagRequest{Name: "Renamed"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", tag.Name)
}

func TestUpdateTag_NotFound(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	repo.On("UpdateTag", mock.Anything, "nonexistent", "user-1", "Test").
		Return(nil, nil)

	tag, err := svc.UpdateTag(context.Background(), "user-1", "nonexistent", &model.UpdateTagRequest{Name: "Test"})
	assert.Nil(t, tag)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
}

// --- DeleteTag Tests ---

func TestDeleteTag_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetTag", mock.Anything, "tag-custom", "user-1").
		Return(makeTag("tag-custom", "Custom", false), nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-custom").
		Return(int64(0), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-custom", "user-1").
		Return(int64(0), nil)
	repo.On("DeleteTag", mock.Anything, "tag-custom", "user-1").
		Return(nil)

	err := svc.DeleteTag(context.Background(), "user-1", "tag-custom")
	require.NoError(t, err)
	repo.AssertCalled(t, "DeleteTag", mock.Anything, "tag-custom", "user-1")
}

func TestDeleteTag_DefaultTagBlocked(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	repo.On("GetTag", mock.Anything, "tag-bills", "user-1").
		Return(makeTag("tag-bills", "Bills", true), nil)

	err := svc.DeleteTag(context.Background(), "user-1", "tag-bills")
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrDefaultTag, svcErr.Code)
	assert.Equal(t, 403, svcErr.Status)
	assert.Contains(t, svcErr.Message, "Default tags cannot be deleted")
}

func TestDeleteTag_InUseByExpenses(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetTag", mock.Anything, "tag-custom", "user-1").
		Return(makeTag("tag-custom", "Custom", false), nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-custom").
		Return(int64(3), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-custom", "user-1").
		Return(int64(0), nil)

	err := svc.DeleteTag(context.Background(), "user-1", "tag-custom")
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrTagInUse, svcErr.Code)
	assert.Equal(t, 409, svcErr.Status)
	assert.Contains(t, svcErr.Message, "3 expense(s)")
}

func TestDeleteTag_InUseByProRataSchedules(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetTag", mock.Anything, "tag-custom", "user-1").
		Return(makeTag("tag-custom", "Custom", false), nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-custom").
		Return(int64(0), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-custom", "user-1").
		Return(int64(2), nil)

	err := svc.DeleteTag(context.Background(), "user-1", "tag-custom")
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrTagInUse, svcErr.Code)
	assert.Contains(t, svcErr.Message, "2 pending schedule(s)")
}

func TestDeleteTag_InUseByBothExpensesAndSchedules(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetTag", mock.Anything, "tag-custom", "user-1").
		Return(makeTag("tag-custom", "Custom", false), nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-custom").
		Return(int64(5), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-custom", "user-1").
		Return(int64(3), nil)

	err := svc.DeleteTag(context.Background(), "user-1", "tag-custom")
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrTagInUse, svcErr.Code)
	assert.Contains(t, svcErr.Message, "5 expense(s)")
	assert.Contains(t, svcErr.Message, "3 pending schedule(s)")
}

func TestDeleteTag_NotFound(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	repo.On("GetTag", mock.Anything, "nonexistent", "user-1").
		Return(nil, nil)

	err := svc.DeleteTag(context.Background(), "user-1", "nonexistent")
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
}
