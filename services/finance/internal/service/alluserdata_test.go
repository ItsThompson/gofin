package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/perf"
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

	// The fan-out issues all three reads concurrently (no serial short-circuit),
	// so the sibling reads must be stubbed even though ListTags is the one failing.
	repo.On("ListTags", mock.Anything, "user-1").Return(nil, fmt.Errorf("db connection failed"))
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{}, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return((*model.DefaultSettings)(nil), nil)

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing tags for export")
}

func TestGetAllUserData_ListPeriodsError(t *testing.T) {
	repo := new(mockRepo)
	svc := newAllUserDataTestService(repo)

	// All three reads fire concurrently; ListPeriods is the failing one.
	repo.On("ListTags", mock.Anything, "user-1").Return([]*model.Tag{}, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").Return(nil, fmt.Errorf("db connection failed"))
	repo.On("GetDefaults", mock.Anything, "user-1").Return((*model.DefaultSettings)(nil), nil)

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

// --- Fan-out regression tests ---

// TestGetAllUserData_FanOutByteIdentical asserts the fan-out assembles exactly
// the serial result over seeded fixtures while reading each source exactly once
// (tags, periods, defaults). It fails if the fan-out drops, duplicates, or
// reorders a read, or diverges from the serial assembly.
func TestGetAllUserData_FanOutByteIdentical(t *testing.T) {
	repo := seedAllUserData(0)
	svc := newFanoutService(repo, nil)

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	require.NoError(t, err)

	expected := &model.AllUserData{
		Tags:     repo.tags,
		Periods:  repo.periods,
		Defaults: repo.defaults,
	}
	assert.Equal(t, expected, result, "fan-out output must be identical to the serial assembly")

	assert.Equal(t, 1, repo.counter.Count("ListTags"), "tags read exactly once")
	assert.Equal(t, 1, repo.counter.Count("ListPeriods"), "periods read exactly once")
	assert.Equal(t, 1, repo.counter.Count("GetDefaults"), "defaults read exactly once")
	assert.Equal(t, 3, repo.counter.Total(), "exactly three reads total")
}

// TestGetAllUserData_FanOutNormalizesNilSlicesAfterBarrier confirms the nil ->
// empty-slice normalization (and nil defaults passthrough) runs after g.Wait(),
// matching the serial version, when every read returns nil. Each source is still
// read exactly once.
func TestGetAllUserData_FanOutNormalizesNilSlicesAfterBarrier(t *testing.T) {
	repo := newCountingAllUserDataRepo() // tags/periods nil, defaults nil
	svc := newFanoutService(repo, nil)

	result, err := svc.GetAllUserData(context.Background(), "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result.Tags)
	assert.Empty(t, result.Tags)
	assert.NotNil(t, result.Periods)
	assert.Empty(t, result.Periods)
	assert.Nil(t, result.Defaults)

	assert.Equal(t, 1, repo.counter.Count("ListTags"))
	assert.Equal(t, 1, repo.counter.Count("ListPeriods"))
	assert.Equal(t, 1, repo.counter.Count("GetDefaults"))
}

// TestGetAllUserData_FanOutRunsConcurrently confirms the three reads overlap
// (fan-out, not serial) while staying within SetLimit(dashboardFanoutLimit).
func TestGetAllUserData_FanOutRunsConcurrently(t *testing.T) {
	repo := seedAllUserData(5 * time.Millisecond)
	svc := newFanoutService(repo, nil)

	_, err := svc.GetAllUserData(context.Background(), "user-1")
	require.NoError(t, err)
	assert.LessOrEqual(t, repo.maxConcurrent(), dashboardFanoutLimit,
		"in-flight reads must not exceed SetLimit(dashboardFanoutLimit)")
	assert.Greater(t, repo.maxConcurrent(), 1, "reads should overlap (fan-out), not run serially")
}

// --- Fan-out test infrastructure ---

// countingAllUserDataRepo is a concurrency-aware fake FinanceRepository for the
// GetAllUserData fan-out regression tests and benchmark. It records one call per
// read through an embedded *perf.CallCounter, tracks the maximum number of reads
// in flight simultaneously (so both the SetLimit bound and real overlap can be
// asserted), and can simulate per-read latency so the benchmark shows fan-out
// (max) rather than serial (sum) wall-clock. Only the three methods
// GetAllUserData reads are implemented; the embedded interface is nil, so any
// other repo call panics and surfaces an accidental extra read. All methods are
// safe for concurrent use.
type countingAllUserDataRepo struct {
	repository.FinanceRepository
	counter  *perf.CallCounter
	delay    time.Duration
	tags     []*model.Tag
	periods  []*model.BudgetPeriod
	defaults *model.DefaultSettings

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
}

func newCountingAllUserDataRepo() *countingAllUserDataRepo {
	return &countingAllUserDataRepo{counter: perf.NewCallCounter()}
}

// enter records the call, bumps the in-flight gauge (tracking the peak), and
// simulates per-read latency. It returns the teardown func the caller defers.
func (r *countingAllUserDataRepo) enter(op string) func() {
	r.counter.Record(op)
	r.mu.Lock()
	r.inFlight++
	if r.inFlight > r.maxInFlight {
		r.maxInFlight = r.inFlight
	}
	r.mu.Unlock()
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return func() {
		r.mu.Lock()
		r.inFlight--
		r.mu.Unlock()
	}
}

func (r *countingAllUserDataRepo) ListTags(context.Context, string) ([]*model.Tag, error) {
	defer r.enter("ListTags")()
	return r.tags, nil
}

func (r *countingAllUserDataRepo) ListPeriods(context.Context, string) ([]*model.BudgetPeriod, error) {
	defer r.enter("ListPeriods")()
	return r.periods, nil
}

func (r *countingAllUserDataRepo) GetDefaults(context.Context, string) (*model.DefaultSettings, error) {
	defer r.enter("GetDefaults")()
	return r.defaults, nil
}

// maxConcurrent reports the peak number of reads observed in flight at once.
func (r *countingAllUserDataRepo) maxConcurrent() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxInFlight
}
