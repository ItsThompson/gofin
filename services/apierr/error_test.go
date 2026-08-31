package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
)

func TestConstructors_SetCodeAndStatus(t *testing.T) {
	cases := []struct {
		name        string
		err         *apierr.Error
		wantCode    string
		wantStatus  int
		wantMessage string
	}{
		{"unauthorized", apierr.Unauthorized("no auth"), apierr.CodeUnauthorized, http.StatusUnauthorized, "no auth"},
		{"not found", apierr.NotFound("gone"), apierr.CodeNotFound, http.StatusNotFound, "gone"},
		{"forbidden", apierr.Forbidden("no access"), apierr.CodeForbidden, http.StatusForbidden, "no access"},
		{"validation", apierr.Validation("bad", nil), apierr.CodeValidation, http.StatusBadRequest, "bad"},
		{"conflict", apierr.Conflict("DUPLICATE_TAG", "dup"), "DUPLICATE_TAG", http.StatusConflict, "dup"},
		{"internal", apierr.Internal("boom"), apierr.CodeInternal, http.StatusInternalServerError, "boom"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.err)
			assert.Equal(t, tc.wantCode, tc.err.Code)
			assert.Equal(t, tc.wantStatus, tc.err.Status)
			assert.Equal(t, tc.wantMessage, tc.err.Message)
		})
	}
}

func TestValidation_CarriesFields(t *testing.T) {
	fields := map[string]string{"amount": "required"}

	err := apierr.Validation("validation failed", fields)

	assert.Equal(t, fields, err.Fields)
}

func TestError_MessageIsErrorString(t *testing.T) {
	err := apierr.NotFound("period not found")

	assert.Equal(t, "period not found", err.Error())
}

func TestError_UnwrapsThroughErrorsAs(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", apierr.Conflict("DUPLICATE_TAG", "dup"))

	var target *apierr.Error
	require.True(t, errors.As(wrapped, &target))
	assert.Equal(t, "DUPLICATE_TAG", target.Code)
	assert.Equal(t, http.StatusConflict, target.Status)
}

// IsServerError decides whether a failure is the service's fault, and callers use
// it to decide whether a failure is worth an error report. A wrong answer either
// hides a real 500 or spends the error budget on client input, so every class is
// pinned here rather than at the call sites.
func TestIsServerError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "an unclassified error renders as 500", err: errors.New("connection refused"), want: true},
		{name: "a wrapped unclassified error", err: fmt.Errorf("insert: %w", errors.New("connection refused")), want: true},
		{name: "an explicit 500", err: apierr.Internal("store unavailable"), want: true},
		{name: "a wrapped 500", err: fmt.Errorf("outer: %w", apierr.Internal("store unavailable")), want: true},
		{name: "a 503", err: &apierr.Error{Code: "SERVICE_UNAVAILABLE", Status: http.StatusServiceUnavailable}, want: true},
		{name: "an unset status falls back to 500", err: &apierr.Error{Code: "NO_STATUS"}, want: true},
		{name: "a negative status falls back to 500", err: &apierr.Error{Code: "ODD", Status: -1}, want: true},
		{name: "a 400", err: apierr.Validation("invalid", nil), want: false},
		{name: "a 401", err: apierr.Unauthorized("no session"), want: false},
		{name: "a 403", err: &apierr.Error{Code: apierr.CodeForbidden, Status: http.StatusForbidden}, want: false},
		{name: "a 404", err: apierr.NotFound("period not found"), want: false},
		{name: "a 409", err: apierr.Conflict("DUPLICATE_TAG", "dup"), want: false},
		{name: "a 429", err: &apierr.Error{Code: "RATE_LIMITED", Status: http.StatusTooManyRequests}, want: false},
		{name: "a wrapped 404", err: fmt.Errorf("outer: %w", apierr.NotFound("gone")), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, apierr.IsServerError(tc.err))
		})
	}
}

// The classifier and the renderer must agree, or a caller reports a failure the
// client never saw as a 5xx, or stays silent on one it did. Derived from the same
// cases rather than a second hand-written list.
func TestIsServerError_AgreesWithWhatRespondWrites(t *testing.T) {
	for _, err := range []error{
		errors.New("connection refused"),
		apierr.Internal("store unavailable"),
		&apierr.Error{Code: "SERVICE_UNAVAILABLE", Status: http.StatusServiceUnavailable},
		&apierr.Error{Code: "NO_STATUS"},
		apierr.Validation("invalid", nil),
		apierr.Unauthorized("no session"),
		apierr.NotFound("period not found"),
		apierr.Conflict("DUPLICATE_TAG", "dup"),
		fmt.Errorf("outer: %w", apierr.NotFound("gone")),
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/test", nil)

		apierr.Respond(c, err)

		assert.Equal(t, w.Code >= http.StatusInternalServerError, apierr.IsServerError(err), err)
	}
}
