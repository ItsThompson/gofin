package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSendUsesWaitAndDisablesMentions(t *testing.T) {
	requestSeen := make(chan Payload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("wait") != "true" {
			t.Errorf("wait query = %q", request.URL.Query().Get("wait"))
		}
		var payload Payload
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		requestSeen <- payload
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	defer server.Close()

	sender, err := NewSender(server.URL, time.Second)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	payload := <-requestSeen
	if payload.Content != "hello" || len(payload.AllowedMentions.Parse) != 0 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestSendLogsRetryAndSafeConfirmation(t *testing.T) {
	var calls atomic.Int32
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	defer server.Close()

	sender, err := NewSender(server.URL+"/secret-token", time.Second, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	sender.Sleep = func(context.Context, time.Duration) error { return nil }
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(logs.String(), `"status_class":"5xx"`) || !strings.Contains(logs.String(), `"message_id":"message-1"`) {
		t.Fatalf("logs missing safe delivery fields: %s", logs.String())
	}
	if strings.Contains(logs.String(), "secret-token") || strings.Contains(logs.String(), server.URL) {
		t.Fatalf("logs expose webhook details: %s", logs.String())
	}
}

func TestSendRetriesOversizedTransientResponses(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusTooManyRequests} {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 1 {
				if status == http.StatusTooManyRequests {
					writer.Header().Set("Retry-After", "0")
				}
				writer.WriteHeader(status)
				_, _ = writer.Write([]byte(strings.Repeat("x", MaxResponseBodyBytes+1)))
				return
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"id":"message-1"}`))
		}))
		sender, err := NewSender(server.URL, time.Second)
		if err != nil {
			t.Fatalf("NewSender: %v", err)
		}
		sender.Sleep = func(context.Context, time.Duration) error { return nil }
		if err := sender.Send(context.Background(), "hello"); err != nil {
			t.Errorf("status %d Send: %v", status, err)
		}
		if calls.Load() != 2 {
			t.Errorf("status %d calls = %d, want 2", status, calls.Load())
		}
		server.Close()
	}
}

func TestSendRetriesOnlyTransientResponses(t *testing.T) {
	var calls atomic.Int32
	var delays []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	defer server.Close()
	sender, _ := NewSender(server.URL, time.Second)
	sender.Sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if calls.Load() != 3 || len(delays) != 2 || delays[0] != time.Second || delays[1] != 2*time.Second {
		t.Fatalf("calls=%d delays=%v", calls.Load(), delays)
	}
}

func TestSendRejectsInvalidConfirmationAndUnexpectedStatus(t *testing.T) {
	for _, status := range []int{
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusPartialContent,
		http.StatusMultipleChoices,
		http.StatusMovedPermanently,
		http.StatusNotModified,
		http.StatusBadRequest,
		http.StatusOK,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
		}))
		sender, _ := NewSender(server.URL, time.Second)
		if err := sender.Send(context.Background(), "hello"); err == nil {
			t.Errorf("status %d succeeded", status)
		}
		server.Close()
	}
}

func TestSendHonorsZeroRetryAfter(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	defer server.Close()
	sender, _ := NewSender(server.URL, time.Second)
	var delays []time.Duration
	sender.Sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(delays) != 1 || delays[0] != 0 {
		t.Fatalf("delays = %v", delays)
	}
}

func TestSendUsesInjectedClockForRetryAfterDate(t *testing.T) {
	base := time.Date(2026, time.September, 7, 9, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			writer.Header().Set("Retry-After", base.Add(5*time.Second).Format(http.TimeFormat))
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	defer server.Close()
	sender, _ := NewSender(server.URL, time.Second)
	sender.Now = func() time.Time { return base }
	var delays []time.Duration
	sender.Sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(delays) != 1 || delays[0] != 5*time.Second {
		t.Fatalf("delays = %v", delays)
	}
}

func TestSendRequiresRetryAfterFor429AndBoundsIt(t *testing.T) {
	for _, retryAfter := range []string{"", "bad", "+1", "-1", "31"} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Retry-After", retryAfter)
			writer.WriteHeader(http.StatusTooManyRequests)
		}))
		sender, _ := NewSender(server.URL, time.Second)
		calls := 0
		sender.Sleep = func(context.Context, time.Duration) error { calls++; return nil }
		err := sender.Send(context.Background(), "hello")
		var sendErr *SendError
		if err == nil || calls != 0 || !errors.As(err, &sendErr) || sendErr.RetryAfterReason == "" {
			t.Errorf("Retry-After %q: err=%v sleeps=%d", retryAfter, err, calls)
		}
		server.Close()
	}
}

func TestSendRetriesPerAttemptTimeoutWhileDeliveryContextLives(t *testing.T) {
	var calls atomic.Int32
	sender := &Sender{
		WebhookURL: "https://discord.example/webhook/token",
		Timeout:    5 * time.Millisecond,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls.Add(1)
			<-request.Context().Done()
			return nil, request.Context().Err()
		})},
		Sleep: func(context.Context, time.Duration) error { return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := sender.Send(ctx, "hello")
	var sendErr *SendError
	if err == nil || !errors.As(err, &sendErr) || sendErr.Kind != "timeout" || sendErr.Attempts != MaxAttempts {
		t.Fatalf("error = %v, want retryable timeout after %d attempts", err, MaxAttempts)
	}
	if calls.Load() != MaxAttempts {
		t.Fatalf("transport calls = %d, want %d", calls.Load(), MaxAttempts)
	}
	if ctx.Err() != nil {
		t.Fatalf("delivery context ended: %v", ctx.Err())
	}
}

func TestSendDoesNotRetryCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		t.Fatal("canceled request reached server")
	}))
	defer server.Close()
	sender, _ := NewSender(server.URL, time.Second)
	calls := 0
	sender.Sleep = func(context.Context, time.Duration) error { calls++; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sender.Send(ctx, "hello"); err == nil || calls != 0 {
		t.Fatalf("err=%v sleeps=%d", err, calls)
	}
}

func TestSendDoesNotExposeWebhookDetails(t *testing.T) {
	token := "secret-webhook-token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	sender, _ := NewSender(server.URL+"/"+token, time.Second)
	sender.Sleep = func(context.Context, time.Duration) error { return nil }
	err := sender.Send(context.Background(), "hello")
	if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("error exposes details: %v", err)
	}
}

func TestNewSenderValidatesScheme(t *testing.T) {
	for _, value := range []string{"ftp://example.com/hook", "://bad", ""} {
		if _, err := NewSender(value, time.Second); err == nil {
			t.Errorf("NewSender(%q) succeeded", value)
		}
	}
}
