package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

func newAllUserDataTestService(repo *mockRepo) *FinanceService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewFinanceService(repo, new(mockTxBeg), logger)
}

func TestGetAllUserData_UserWithData(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	now := time.Now()
	tags := []*model.Tag{
		{ID: "tag-1", UserID: "user-1", Name: "Bills", IsDefault: true, CreatedAt: now},
		{ID: "tag-2", UserID: "user-1", Name: "Food", IsDefault: true, CreatedAt: now},
		{ID: "tag-3", UserID: "user-1", Name: "Custom", IsDefault: false, CreatedAt: now},
	}
	periods := []*model.BudgetPeriod{
		{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 500000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20, CreatedAt: now},
		{ID: "period-2", UserID: "user-1", Year: 2026, Month: 2, BudgetAmount: 500000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20, CreatedAt: now},
	}
	defaults := &model.DefaultSettings{
		UserID:            "user-1",
		BudgetAmount:      500000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "GBP",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	repo.On("ListTags", mock.Anything, "user-1").Return(tags, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return(defaults, nil)

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, result.Tags, 3)
	assert.Len(t, result.Periods, 2)
	assert.NotNil(t, result.Defaults)
	assert.Equal(t, "GBP", result.Defaults.Currency)
	assert.Equal(t, "Bills", result.Tags[0].Name)
	assert.Equal(t, int32(2026), result.Periods[0].Year)
}

func TestGetAllUserData_UserWithOnlyDefaults(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	now := time.Now()
	defaults := &model.DefaultSettings{
		UserID:            "user-2",
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "USD",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	// User has no tags and no periods
	repo.On("ListTags", mock.Anything, "user-2").Return([]*model.Tag{}, nil)
	repo.On("ListPeriods", mock.Anything, "user-2").Return([]*model.BudgetPeriod{}, nil)
	repo.On("GetDefaults", mock.Anything, "user-2").Return(defaults, nil)

	result, err := svc.GetAllUserData(context.Background(), "user-2")
	require.NoError(t, err)
	assert.Empty(t, result.Tags)
	assert.Empty(t, result.Periods)
	assert.NotNil(t, result.Defaults)
	assert.Equal(t, "USD", result.Defaults.Currency)
}

func TestGetAllUserData_UserWithNoData(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	// User has nothing: no tags, no periods, no defaults
	repo.On("ListTags", mock.Anything, "user-3").Return(nil, nil)
	repo.On("ListPeriods", mock.Anything, "user-3").Return(nil, nil)
	repo.On("GetDefaults", mock.Anything, "user-3").Return(nil, nil)

	result, err := svc.GetAllUserData(context.Background(), "user-3")
	require.NoError(t, err)
	// Should return empty slices, not nil
	assert.NotNil(t, result.Tags)
	assert.NotNil(t, result.Periods)
	assert.Empty(t, result.Tags)
	assert.Empty(t, result.Periods)
	assert.Nil(t, result.Defaults)
}

func TestGetAllUserData_CreatedAtPopulated(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	createdAt := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	tags := []*model.Tag{
		{ID: "tag-1", UserID: "user-1", Name: "Bills", IsDefault: true, CreatedAt: createdAt},
	}
	periods := []*model.BudgetPeriod{
		{ID: "period-1", UserID: "user-1", Year: 2026, Month: 3, BudgetAmount: 400000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20, CreatedAt: createdAt},
	}

	repo.On("ListTags", mock.Anything, "user-1").Return(tags, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return((*model.DefaultSettings)(nil), nil)

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, createdAt, result.Tags[0].CreatedAt)
	assert.Equal(t, createdAt, result.Periods[0].CreatedAt)
}

func TestGetAllUserData_ListTagsError(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	repo.On("ListTags", mock.Anything, "user-1").Return(nil, fmt.Errorf("db connection failed"))

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing tags for export")
}

func TestGetAllUserData_ListPeriodsError(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	repo.On("ListTags", mock.Anything, "user-1").Return([]*model.Tag{}, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").Return(nil, fmt.Errorf("db connection failed"))

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing periods for export")
}

func TestGetAllUserData_GetDefaultsError(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	repo.On("ListTags", mock.Anything, "user-1").Return([]*model.Tag{}, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{}, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return(nil, fmt.Errorf("db connection failed"))

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting defaults for export")
}
