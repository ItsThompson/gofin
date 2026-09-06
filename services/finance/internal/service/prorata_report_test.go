package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// A MarkProRataApplied failure is swallowed (the ledger write succeeded), so
// the service is the failure's only reporter: one event naming the operation
// and the schedule, while the schedule stays unapplied exactly as before.
func TestApplyPendingProRata_MarkAppliedFailure_YieldsOneServiceOwnedEvent(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-prev", 2026, 4), nil)
	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).
		Return(makePeriod("p-1", 2026, 5), nil)

	pendingSchedule := &model.ProRataSchedule{
		ID: "sched-1", UserID: "user-1", ProRataGroup: "group-1",
		Name: "Insurance", InstallmentAmountInMinorUnits: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 5, InstallmentIndex: 2,
		InstallmentTotal: 3, Status: "pending",
		TransactionCurrencyCode: "USD", CapturedRateSnapshot: snapshotFixture(),
	}
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{pendingSchedule}, nil)
	expClient.On("CreateProRataInstallment", mock.Anything, mock.Anything).
		Return(&CreatedExpenseData{ID: "exp-applied", CreatedAt: "2026-05-01T00:00:00Z"}, nil)
	repo.On("MarkProRataApplied", mock.Anything, "sched-1").
		Return(errors.New("connection refused"))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	result, err := svc.CreatePeriodWithProRata(ctx, "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrencyCode: "USD",
	})

	require.NoError(t, err, "the period is still created and returned")
	assert.Empty(t, result.AppliedProRata, "a schedule whose status write failed is not applied")

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "finance.prorata_apply", events[0].Tags["operation"])
	assert.Equal(t, "budgets", events[0].Tags["domain"])
	assert.Equal(t, "sched-1", events[0].Contexts["gofin"]["schedule_id"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, "connection refused")
}

// A MarkProRataFailed failure inside markProRataFailed is swallowed too (the
// repo is already broken), so it reports service-side instead of vanishing.
func TestMarkProRataFailed_RepoFailure_YieldsOneServiceOwnedEvent(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-prev", 2026, 4), nil)
	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).
		Return(makePeriod("p-1", 2026, 5), nil)

	// A snapshot that cannot derive the target reporting currency drives the
	// deterministic markProRataFailed path; the status write itself then fails.
	pendingSchedule := &model.ProRataSchedule{
		ID: "sched-2", UserID: "user-1", ProRataGroup: "group-1",
		Name: "Insurance", InstallmentAmountInMinorUnits: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 5, InstallmentIndex: 2,
		InstallmentTotal: 3, Status: "pending",
		TransactionCurrencyCode: "USD", CapturedRateSnapshot: snapshotFixture(),
	}
	// The period's reporting currency (USD) is absent from the snapshot, so the
	// pre-write coverage check marks the schedule failed deterministically.
	delete(pendingSchedule.CapturedRateSnapshot.RatesByCurrency, "USD")

	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{pendingSchedule}, nil)
	repo.On("MarkProRataFailed", mock.Anything, "sched-2", mock.Anything).
		Return(errors.New("connection refused"))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	result, err := svc.CreatePeriodWithProRata(ctx, "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrencyCode: "USD",
	})

	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "finance.prorata_apply", events[0].Tags["operation"])
	assert.Equal(t, "sched-2", events[0].Contexts["gofin"]["schedule_id"])
	assert.Equal(t, model.ErrSnapshotCurrencyMissing, events[0].Contexts["gofin"]["intended_failure_reason"])
}
