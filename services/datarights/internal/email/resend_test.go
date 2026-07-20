package email

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResendSender_HTTPInteraction_Success(t *testing.T) {
	var capturedReq resendRequest
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedReq))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "email-abc-123"}`))
	}))
	defer server.Close()

	tokens := testTokens()
	sender, err := NewResendSender("re_test_key_123", "gofin <noreply@usegofin.com>", tokens, testLogger())
	require.NoError(t, err)

	sender.httpClient = server.Client()
	// ResendSender hardcodes the Resend URL; rewrite it to the test server.
	sender.httpClient.Transport = &rewriteTransport{
		base:    server.Client().Transport,
		baseURL: server.URL,
	}

	zipBytes := []byte("fake-zip-content-for-testing")
	err = sender.SendExportEmail(context.Background(), "user@example.com", zipBytes)
	require.NoError(t, err)

	// Verify authorization header
	assert.Equal(t, "Bearer re_test_key_123", capturedAuth)

	// Verify request body
	assert.Equal(t, "gofin <noreply@usegofin.com>", capturedReq.From)
	assert.Equal(t, []string{"user@example.com"}, capturedReq.To)
	assert.Equal(t, "Your gofin data export is ready", capturedReq.Subject)
	assert.NotEmpty(t, capturedReq.HTML)
	assert.NotEmpty(t, capturedReq.Text)
	assert.Contains(t, capturedReq.HTML, "#f1f5f9") // brand color in HTML

	// Verify attachment
	require.Len(t, capturedReq.Attachments, 1)
	assert.Contains(t, capturedReq.Attachments[0].Filename, "gofin-export-")
	assert.Contains(t, capturedReq.Attachments[0].Filename, ".zip")
	assert.NotEmpty(t, capturedReq.Attachments[0].Content)
}

func TestResendSender_HTTPInteraction_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message": "rate limit exceeded"}`))
	}))
	defer server.Close()

	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "from@example.com", tokens, testLogger())
	require.NoError(t, err)

	sender.httpClient = server.Client()
	sender.httpClient.Transport = &rewriteTransport{
		base:    server.Client().Transport,
		baseURL: server.URL,
	}

	err = sender.SendExportEmail(context.Background(), "user@example.com", []byte("zip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resend API error (status 429)")
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestResendSender_HTTPInteraction_NetworkError(t *testing.T) {
	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "from@example.com", tokens, testLogger())
	require.NoError(t, err)

	// Point at a server that doesn't exist
	sender.httpClient.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		baseURL: "http://localhost:1", // port 1 won't be listening
	}

	err = sender.SendExportEmail(context.Background(), "user@example.com", []byte("zip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sending email via Resend")
}

func TestResendSender_EmptyEmail_ReturnsError(t *testing.T) {
	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "from@example.com", tokens, testLogger())
	require.NoError(t, err)

	err = sender.SendExportEmail(context.Background(), "", []byte("zip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient email address is empty")
}

func TestResendSender_ContextCancelled(t *testing.T) {
	// Server that never responds (blocks)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "from@example.com", tokens, testLogger())
	require.NoError(t, err)

	sender.httpClient = server.Client()
	sender.httpClient.Transport = &rewriteTransport{
		base:    server.Client().Transport,
		baseURL: server.URL,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err = sender.SendExportEmail(ctx, "user@example.com", []byte("zip"))
	require.Error(t, err)
}

// rewriteTransport intercepts requests and rewrites the URL to point at a test server.
type rewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point at the test server, preserving path
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[len("http://"):]
	return t.base.RoundTrip(req)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
