package discord

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
	digitsOnly := true
	for _, char := range value {
		if char < '0' || char > '9' {
			digitsOnly = false
			break
		}
	}
	if digitsOnly {
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return 0, retryAfterMalformed
		}
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
