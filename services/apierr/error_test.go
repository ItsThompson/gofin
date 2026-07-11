package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

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
