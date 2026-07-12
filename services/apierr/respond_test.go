package apierr_test

import (
	"encoding/json"
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

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestContext returns a gin context wired to a fresh recorder so Respond
// can be exercised without a full router.
func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// decodeBody unmarshals the recorded response into a generic map so tests can
// assert exact wire keys, including the omitempty behavior of "fields".
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestRespond_CodeToStatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        *apierr.Error
		wantStatus int
		wantCode   string
	}{
		{"unauthorized", apierr.Unauthorized("nope"), http.StatusUnauthorized, apierr.CodeUnauthorized},
		{"not found", apierr.NotFound("missing"), http.StatusNotFound, apierr.CodeNotFound},
		{"validation", apierr.Validation("bad", nil), http.StatusBadRequest, apierr.CodeValidation},
		{"conflict", apierr.Conflict("DUPLICATE_TAG", "dup"), http.StatusConflict, "DUPLICATE_TAG"},
		{"internal", apierr.Internal("boom"), http.StatusInternalServerError, apierr.CodeInternal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newTestContext()

			apierr.Respond(c, tc.err)

			require.Equal(t, tc.wantStatus, w.Code)
			body := decodeBody(t, w)
			assert.Equal(t, tc.wantCode, body["code"])
			assert.Equal(t, tc.err.Message, body["message"])
		})
	}
}

func TestRespond_CopiesFieldsOntoWire(t *testing.T) {
	c, w := newTestContext()
	fields := map[string]string{"amount": "required", "date": "invalid"}

	apierr.Respond(c, apierr.Validation("validation failed", fields))

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, apierr.CodeValidation, body["code"])

	gotFields, ok := body["fields"].(map[string]any)
	require.True(t, ok, "fields must be present on the wire response")
	assert.Equal(t, "required", gotFields["amount"])
	assert.Equal(t, "invalid", gotFields["date"])
}

func TestRespond_OmitsFieldsWhenAbsent(t *testing.T) {
	c, w := newTestContext()

	apierr.Respond(c, apierr.NotFound("missing"))

	body := decodeBody(t, w)
	_, present := body["fields"]
	assert.False(t, present, "fields must be omitted when nil (omitempty)")
}

func TestRespond_WrappedErrorClassifiedByErrorsAs(t *testing.T) {
	c, w := newTestContext()
	wrapped := fmt.Errorf("repository call failed: %w", apierr.NotFound("period not found"))

	apierr.Respond(c, wrapped)

	require.Equal(t, http.StatusNotFound, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, apierr.CodeNotFound, body["code"])
	assert.Equal(t, "period not found", body["message"])
}

func TestRespond_UnknownErrorYields500Internal(t *testing.T) {
	c, w := newTestContext()

	apierr.Respond(c, errors.New("some unexpected failure"))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, apierr.CodeInternal, body["code"])
	assert.Equal(t, "An unexpected error occurred", body["message"])
}

func TestRespond_ZeroStatusErrorYields500Internal(t *testing.T) {
	c, w := newTestContext()
	// A hand-built *Error with an unset Status (0) must not yield WriteHeader(0);
	// Respond falls back to a coherent 500 INTERNAL_SERVER_ERROR.
	malformed := &apierr.Error{Code: "SOMETHING", Message: "no status set"}

	apierr.Respond(c, malformed)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, apierr.CodeInternal, body["code"])
	assert.Equal(t, "An unexpected error occurred", body["message"])
}
