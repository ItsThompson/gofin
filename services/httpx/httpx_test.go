package httpx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/httpx"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func newContextWithRequest(req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestRequireUserID_PresentHeaderReturnsValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/finance", nil)
	req.Header.Set("X-User-ID", "user-123")
	c, w := newContextWithRequest(req)

	userID, ok := httpx.RequireUserID(c)

	assert.True(t, ok)
	assert.Equal(t, "user-123", userID)
	assert.Equal(t, http.StatusOK, w.Code, "no error response should be written")
}

func TestRequireUserID_MissingHeaderWrites401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/finance", nil)
	c, w := newContextWithRequest(req)

	userID, ok := httpx.RequireUserID(c)

	assert.False(t, ok)
	assert.Empty(t, userID)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, apierr.CodeUnauthorized, decodeBody(t, w)["code"])
}

func TestRequireUserID_EmptyHeaderWrites401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/finance", nil)
	req.Header.Set("X-User-ID", "")
	c, w := newContextWithRequest(req)

	_, ok := httpx.RequireUserID(c)

	assert.False(t, ok)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, apierr.CodeUnauthorized, decodeBody(t, w)["code"])
}

type createExpenseRequest struct {
	Amount int    `json:"amount" binding:"required"`
	Note   string `json:"note"`
}

func newJSONContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/api/expense", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return newContextWithRequest(req)
}

func TestBindJSON_ValidBodyPopulatesDst(t *testing.T) {
	c, w := newJSONContext(`{"amount": 42, "note": "lunch"}`)

	var req createExpenseRequest
	ok := httpx.BindJSON(c, &req)

	assert.True(t, ok)
	assert.Equal(t, 42, req.Amount)
	assert.Equal(t, "lunch", req.Note)
	assert.Equal(t, http.StatusOK, w.Code, "no error response should be written")
}

func TestBindJSON_MalformedBodyWrites400(t *testing.T) {
	c, w := newJSONContext(`{"amount": `)

	var req createExpenseRequest
	ok := httpx.BindJSON(c, &req)

	assert.False(t, ok)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, apierr.CodeValidation, decodeBody(t, w)["code"])
}

func TestBindJSON_ValidationFailureAttachesFieldDetail(t *testing.T) {
	// "amount" is required; omitting it triggers validator.ValidationErrors.
	c, w := newJSONContext(`{"note": "lunch"}`)

	var req createExpenseRequest
	ok := httpx.BindJSON(c, &req)

	assert.False(t, ok)
	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, apierr.CodeValidation, body["code"])

	fields, present := body["fields"].(map[string]any)
	require.True(t, present, "validator failures must surface field detail")
	assert.Equal(t, "required", fields["Amount"])
}
