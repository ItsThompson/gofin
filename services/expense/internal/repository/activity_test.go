package repository

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type activityQueryClient struct {
	query  string
	params map[string]interface{}
	result *SQLResult
	err    error
}

func (c *activityQueryClient) SQLExec(context.Context, string, map[string]interface{}) (*SQLResult, error) {
	return &SQLResult{}, nil
}

func (c *activityQueryClient) SQLQuery(_ context.Context, query string, params map[string]interface{}) (*SQLResult, error) {
	c.query = query
	c.params = params
	return c.result, c.err
}

func activityWindowSet() *reportingpb.ReportWindowSet {
	reportStart := time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC)
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart),
			End:   timestamppb.New(reportStart.AddDate(0, 0, 7)),
		},
		PreviousWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart.AddDate(0, 0, -7)),
			End:   timestamppb.New(reportStart),
		},
		TrailingFourWeeks: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart.AddDate(0, 0, -28)),
			End:   timestamppb.New(reportStart),
		},
	}
}

func TestCountTotal_UsesCreatedAtWindowsAndMapsAllCounts(t *testing.T) {
	rows := make([]SQLRow, 0, 20)
	for i := 0; i < 4; i++ {
		rows = append(rows, SQLRow{Values: []SQLValue{fakeSQLValue{stringValue: "2026-05-05T00:00:00Z"}}})
	}
	for i := 0; i < 3; i++ {
		rows = append(rows, SQLRow{Values: []SQLValue{fakeSQLValue{stringValue: "2026-04-28T00:00:00Z"}}})
	}
	for i := 0; i < 10; i++ {
		rows = append(rows, SQLRow{Values: []SQLValue{fakeSQLValue{stringValue: "2026-04-07T00:00:00Z"}}})
	}
	client := &activityQueryClient{result: &SQLResult{Rows: rows}}
	repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	counts, err := repo.CountTotal(context.Background(), activityWindowSet())

	require.NoError(t, err)
	assert.Equal(t, &ActivityMetricCounts{ReportWeek: 4, PreviousWeek: 3, TrailingFourWeeksTotal: 13}, counts)
	assert.Contains(t, client.query, "SELECT created_at")
	assert.Contains(t, client.query, "created_at >= @trailing_four_weeks_start")
	assert.Contains(t, client.query, "created_at < @report_week_end")
	assert.NotContains(t, client.query, "GROUP BY")
	assert.Equal(t, "2026-04-06T00:00:00Z", client.params["trailing_four_weeks_start"])
	assert.Equal(t, "2026-05-11T00:00:00Z", client.params["report_week_end"])
}

func TestActivityMetricCounts_ApplyClassificationPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		count     func(*ImmudbExpenseRepository, context.Context, *reportingpb.ReportWindowSet) (*ActivityMetricCounts, error)
		predicate string
	}{
		{
			name:      "manual",
			count:     (*ImmudbExpenseRepository).CountManual,
			predicate: "corrects_id IS NULL OR corrects_id = ''",
		},
		{
			name:      "corrections",
			count:     (*ImmudbExpenseRepository).CountCorrections,
			predicate: "corrects_id IS NOT NULL AND corrects_id <> ''",
		},
		{
			name:      "pro-rata",
			count:     (*ImmudbExpenseRepository).CountProRata,
			predicate: "corrects_id IS NULL OR corrects_id = ''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &activityQueryClient{result: &SQLResult{Rows: []SQLRow{{Values: []SQLValue{
				fakeSQLValue{stringValue: "2026-05-05T00:00:00Z"},
			}}}}}
			repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))

			_, err := tt.count(repo, context.Background(), activityWindowSet())

			require.NoError(t, err)
			assert.Contains(t, client.query, tt.predicate)
			assert.Contains(t, client.query, "status <> @redacted_status")
			if tt.name == "manual" {
				assert.Contains(t, client.query, "is_pro_rata = false")
			}
			if tt.name == "pro-rata" {
				assert.Contains(t, client.query, "is_pro_rata = true")
			}
		})
	}
}

func TestCountActive_UsesCurrentActiveStatus(t *testing.T) {
	client := &activityQueryClient{result: &SQLResult{Rows: []SQLRow{{Values: []SQLValue{
		fakeSQLValue{stringValue: "2026-05-05T00:00:00Z"},
	}}}}}
	repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	_, err := repo.CountActive(context.Background(), activityWindowSet())

	require.NoError(t, err)
	assert.Contains(t, client.query, "status = @active_status")
	assert.Equal(t, "active", client.params["active_status"])
}

func TestCountTotal_PropagatesQueryFailure(t *testing.T) {
	client := &activityQueryClient{err: errors.New("query failed")}
	repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	_, err := repo.CountTotal(context.Background(), activityWindowSet())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "counting activity metric")
	assert.Contains(t, err.Error(), "query failed")
}
