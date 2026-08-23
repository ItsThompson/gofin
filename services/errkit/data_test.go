package errkit_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/errkit"
)

// dataCarrierError is a typed error that carries report data, the shape a
// service error implements so a handler report does not repeat the context.
type dataCarrierError struct {
	msg  string
	data map[string]any
}

func (e dataCarrierError) Error() string { return e.msg }

func (e dataCarrierError) ReportData() map[string]any { return e.data }

var _ errkit.DataCarrier = dataCarrierError{}

func TestReport_MergesDataCarrierDataIntoContextAndLog(t *testing.T) {
	env := newReportEnv(t)
	err := dataCarrierError{
		msg:  "budget over limit",
		data: map[string]any{"budget_id": "b-1", "attempt": 2},
	}

	_ = errkit.Report(env.ctx, err, errkit.Meta{Op: "budget.check"})

	event := env.singleEvent(t)
	assert.Equal(t, sentry.Context{"budget_id": "b-1", "attempt": 2}, event.Contexts["gofin"])

	record := env.singleRecord(t)
	assert.Equal(t, map[string]any{
		"error":      "budget over limit",
		"error_kind": "internal",
		"operation":  "budget.check",
		"budget_id":  "b-1",
		"attempt":    int64(2),
	}, recordAttrs(record))
}

func TestReport_CallerDataWinsOverDataCarrierOnCollision(t *testing.T) {
	env := newReportEnv(t)
	err := dataCarrierError{
		msg:  "boom",
		data: map[string]any{"expense_id": "from-error", "shared": "error"},
	}

	_ = errkit.Report(env.ctx, err, errkit.Meta{
		Op:   "expense.create",
		Data: map[string]any{"expense_id": "from-caller", "caller_only": true},
	})

	event := env.singleEvent(t)
	assert.Equal(t, sentry.Context{
		"expense_id":  "from-caller",
		"shared":      "error",
		"caller_only": true,
	}, event.Contexts["gofin"])

	record := env.singleRecord(t)
	attrs := recordAttrs(record)
	assert.Equal(t, "from-caller", attrs["expense_id"])
	assert.Equal(t, "error", attrs["shared"])
	assert.Equal(t, true, attrs["caller_only"])
}

func TestReport_FindsDataCarrierThroughWrapChain(t *testing.T) {
	env := newReportEnv(t)
	carrier := dataCarrierError{msg: "budget over limit", data: map[string]any{"budget_id": "b-1"}}
	err := fmt.Errorf("check budget: %w", carrier)

	_ = errkit.Report(env.ctx, err, errkit.Meta{Op: "budget.check"})

	event := env.singleEvent(t)
	assert.Equal(t, sentry.Context{"budget_id": "b-1"}, event.Contexts["gofin"])

	record := env.singleRecord(t)
	assert.Equal(t, "b-1", recordAttrs(record)["budget_id"])
}

func TestReport_NonDataCarrierLeavesDataEmpty(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Op: "expense.create"})

	event := env.singleEvent(t)
	assert.NotContains(t, event.Contexts, "gofin")

	record := env.singleRecord(t)
	assert.Equal(t, map[string]any{
		"error":      "boom",
		"error_kind": "internal",
		"operation":  "expense.create",
	}, recordAttrs(record))
}
