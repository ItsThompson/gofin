package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
}

func NewSender(webhookURL string, timeout time.Duration) (*Sender, error) {
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
	return &Sender{
		WebhookURL: webhookURL,
		Timeout:    timeout,
		Client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Sleep: sleepContext,
		Now:   time.Now,
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
		err := s.sendAttempt(ctx, endpoint, body)
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

func (s *Sender) sendAttempt(ctx context.Context, endpoint string, body []byte) error {
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
	limited := io.LimitReader(response.Body, MaxResponseBodyBytes+1)
	responseBody, readErr := io.ReadAll(limited)
	if readErr != nil {
		return &SendError{Kind: "response", StatusCode: response.StatusCode}
	}
	if len(responseBody) > MaxResponseBodyBytes {
		return &SendError{Kind: "response_too_large", StatusCode: response.StatusCode}
	}
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
	var confirmation struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBody, &confirmation); err != nil || strings.TrimSpace(confirmation.ID) == "" {
		return &SendError{Kind: "invalid_confirmation", StatusCode: response.StatusCode}
	}
	return nil
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

type retryAfterState uint8

const (
	retryAfterMissing retryAfterState = iota
	retryAfterValid
	retryAfterMalformed
	retryAfterNegative
	retryAfterTooLong
)

func parseRetryAfterValue(value string, now time.Time) (time.Duration, retryAfterState) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, retryAfterMissing
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, retryAfterNegative
		}
		if seconds > int(MaxRetryAfter/time.Second) {
			return 0, retryAfterTooLong
		}
		delay := time.Duration(seconds) * time.Second
		if delay > MaxRetryAfter {
			return 0, retryAfterTooLong
		}
		return delay, retryAfterValid
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, retryAfterMalformed
	}
	if !when.After(now) {
		return 0, retryAfterValid
	}
	delay := when.Sub(now)
	if delay > MaxRetryAfter {
		return 0, retryAfterTooLong
	}
	return delay, retryAfterValid
}

func retryAfterReason(state retryAfterState) string {
	switch state {
	case retryAfterMissing:
		return "missing"
	case retryAfterMalformed:
		return "malformed"
	case retryAfterNegative:
		return "negative"
	case retryAfterTooLong:
		return "over_limit"
	default:
		return "invalid"
	}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type ConfigError struct {
	Kind string
}

func (e *ConfigError) Error() string {
	if e == nil {
		return ""
	}
	return "discord configuration failed: " + e.Kind
}

type SendError struct {
	Kind             string
	StatusCode       int
	Retryable        bool
	RetryAfter       time.Duration
	RetryAfterSet    bool
	RetryAfterReason string
	Attempts         int
}

func (e *SendError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf("discord delivery failed: %s", e.Kind)
	if e.StatusCode != 0 {
		message += " (HTTP " + strconv.Itoa(e.StatusCode) + ")"
	}
	if e.RetryAfterReason != "" {
		message += " (" + e.RetryAfterReason + ")"
	}
	if e.Attempts > 0 {
		message += ", attempts=" + strconv.Itoa(e.Attempts)
	}
	return message
}
