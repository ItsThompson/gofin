package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	MaxResponseBodyBytes = 64 * 1024
	MaxRetryAfter        = 30 * time.Second
	MaxAttempts          = 3
)

type Payload struct {
	Content         string          `json:"content"`
	AllowedMentions AllowedMentions `json:"allowed_mentions"`
}

type AllowedMentions struct {
	Parse []string `json:"parse"`
}

type Sender struct {
	WebhookURL string
	Client     *http.Client
	Timeout    time.Duration
	Sleep      func(context.Context, time.Duration) error
	Now        func() time.Time
	logger     *slog.Logger
}

func NewSender(webhookURL string, timeout time.Duration, logger ...*slog.Logger) (*Sender, error) {
	if strings.TrimSpace(webhookURL) == "" {
		return nil, &ConfigError{Kind: "missing_webhook"}
	}
	parsed, err := url.Parse(webhookURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, &ConfigError{Kind: "invalid_webhook"}
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var senderLogger *slog.Logger
	if len(logger) > 0 {
		senderLogger = logger[0]
	}
	return &Sender{
		WebhookURL: webhookURL,
		Timeout:    timeout,
		Client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Sleep:  sleepContext,
		Now:    time.Now,
		logger: senderLogger,
	}, nil
}

func (s *Sender) Send(ctx context.Context, report string) error {
	if s == nil || s.Client == nil {
		return &SendError{Kind: "configuration"}
	}
	endpoint, err := webhookEndpoint(s.WebhookURL)
	if err != nil {
		return &SendError{Kind: "configuration"}
	}
	body, err := json.Marshal(Payload{Content: report, AllowedMentions: AllowedMentions{Parse: []string{}}})
	if err != nil {
		return &SendError{Kind: "request"}
	}

	sleep := s.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	var last *SendError
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		err := s.sendAttempt(ctx, endpoint, body, attempt)
		s.logAttempt(attempt, err)
		if err == nil {
			return nil
		}
		last, _ = err.(*SendError)
		if last == nil || !last.Retryable || attempt == MaxAttempts {
			if last != nil {
				last.Attempts = attempt
			}
			return err
		}
		last.Attempts = attempt
		delay := last.RetryAfter
		if delay <= 0 && !last.RetryAfterSet {
			delay = time.Duration(attempt) * time.Second
		}
		if delay > MaxRetryAfter {
			delay = MaxRetryAfter
		}
		if err := sleep(ctx, delay); err != nil {
			return &SendError{Kind: "canceled", Attempts: attempt}
		}
	}
	return last
}

func (s *Sender) sendAttempt(ctx context.Context, endpoint string, body []byte, attempt int) error {
	attemptContext := ctx
	cancel := func() {}
	if s.Timeout > 0 {
		attemptContext, cancel = context.WithTimeout(ctx, s.Timeout)
	}
	defer cancel()
	request, err := http.NewRequestWithContext(attemptContext, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &SendError{Kind: "request"}
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.Client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return &SendError{Kind: "canceled"}
		}
		if attemptContext.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			return &SendError{Kind: "timeout", Retryable: true}
		}
		return &SendError{Kind: "network", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		retryable := response.StatusCode >= 500 && response.StatusCode <= 599
		retryAfter := time.Duration(0)
		if response.StatusCode == http.StatusTooManyRequests {
			var state retryAfterState
			now := time.Now
			if s.Now != nil {
				now = s.Now
			}
			retryAfter, state = parseRetryAfterValue(response.Header.Get("Retry-After"), now())
			if state != retryAfterValid {
				return &SendError{
					Kind:             "invalid_retry_after",
					StatusCode:       response.StatusCode,
					RetryAfterReason: retryAfterReason(state),
				}
			}
			retryable = true
		}
		sendError := &SendError{
			Kind:       "http",
			StatusCode: response.StatusCode,
			Retryable:  retryable,
			RetryAfter: retryAfter,
		}
		if response.StatusCode == http.StatusTooManyRequests {
			sendError.RetryAfterSet = true
		}
		return sendError
	}
	limited := io.LimitReader(response.Body, MaxResponseBodyBytes+1)
	responseBody, readErr := io.ReadAll(limited)
	if readErr != nil {
		return &SendError{Kind: "response", StatusCode: response.StatusCode}
	}
	if len(responseBody) > MaxResponseBodyBytes {
		return &SendError{Kind: "response_too_large", StatusCode: response.StatusCode}
	}
	var confirmation struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBody, &confirmation); err != nil || strings.TrimSpace(confirmation.ID) == "" {
		return &SendError{Kind: "invalid_confirmation", StatusCode: response.StatusCode}
	}
	if s.logger != nil {
		s.logger.Info("reporting delivery confirmation received",
			slog.String("service", "reporting"),
			slog.String("component", "discord"),
			slog.String("outcome", "confirmed"),
			slog.Int("attempt", attempt),
			slog.String("message_id", safeMessageID(confirmation.ID)),
		)
	}
	return nil
}

func safeMessageID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return "redacted"
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return "redacted"
		}
	}
	return value
}

func (s *Sender) logAttempt(attempt int, err error) {
	if s.logger == nil {
		return
	}
	attrs := []any{
		slog.String("service", "reporting"),
		slog.String("component", "discord"),
		slog.Int("attempt", attempt),
	}
	if err == nil {
		s.logger.Info("reporting delivery attempt completed", append(attrs, slog.String("outcome", "success"))...)
		return
	}
	var sendErr *SendError
	if errors.As(err, &sendErr) {
		attrs = append(attrs,
			slog.String("outcome", "failure"),
			slog.String("code", sendErr.Kind),
			slog.String("status_class", statusClass(sendErr.StatusCode)),
			slog.Bool("retryable", sendErr.Retryable),
		)
	} else {
		attrs = append(attrs, slog.String("outcome", "failure"), slog.String("code", "unknown"))
	}
	s.logger.Warn("reporting delivery attempt failed", attrs...)
}

func statusClass(statusCode int) string {
	if statusCode < 100 {
		return "none"
	}
	return fmt.Sprintf("%dxx", statusCode/100)
}

func webhookEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid webhook URL")
	}
	query := parsed.Query()
	query.Set("wait", "true")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
