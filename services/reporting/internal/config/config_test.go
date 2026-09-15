package config

import (
	"testing"
	"time"
)

func TestParseReportDateRequiresMonday(t *testing.T) {
	if _, err := ParseReportDate("2026-09-08"); err == nil {
		t.Fatal("expected non-Monday date to fail")
	}
	parsed, err := ParseReportDate("2026-09-07")
	if err != nil {
		t.Fatalf("ParseReportDate: %v", err)
	}
	if got := parsed.Location(); got != time.UTC {
		t.Fatalf("location = %v, want UTC", got)
	}
}

func TestParseReportDateRejectsNonCanonicalInput(t *testing.T) {
	for _, value := range []string{"2026-9-7", "2026-09-07T00:00:00Z", "2026-02-30"} {
		if _, err := ParseReportDate(value); err == nil {
			t.Errorf("ParseReportDate(%q) succeeded", value)
		}
	}
}

func TestLatestCompletedMondayUsesUTC(t *testing.T) {
	now := time.Date(2026, time.September, 9, 23, 30, 0, 0, time.FixedZone("local", -7*60*60))
	got := LatestCompletedMonday(now)
	want := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("LatestCompletedMonday = %s, want %s", got, want)
	}
}

func TestResolveReportDateDefaultsToLatestCompletedWeek(t *testing.T) {
	now := time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC)
	got, err := ResolveReportDate("", now)
	want := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	if err != nil || !got.Equal(want) {
		t.Fatalf("default report week = %s, err=%v", got, err)
	}
}

func TestResolveReportDateRejectsIncompleteExplicitWeek(t *testing.T) {
	now := time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC)
	if _, err := ResolveReportDate("2026-09-14", now); err == nil {
		t.Fatal("expected current week to be rejected")
	}
	if got, err := ResolveReportDate("2026-09-07", now); err != nil || !got.Equal(time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("completed week = %s, err=%v", got, err)
	}
}

func TestFixedTimeoutPolicy(t *testing.T) {
	if DefaultRPCTimeout != 10*time.Second || DefaultCollectionTimeout != 15*time.Second || DefaultDiscordTimeout != 10*time.Second || DefaultDeliveryTimeout != 60*time.Second || DefaultAppTimeout != 90*time.Second {
		t.Fatalf("timeout policy changed: rpc=%s collection=%s discord=%s delivery=%s app=%s", DefaultRPCTimeout, DefaultCollectionTimeout, DefaultDiscordTimeout, DefaultDeliveryTimeout, DefaultAppTimeout)
	}
}
