package collector

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	authpb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
	datarightspb "github.com/ItsThompson/gofin/services/datarights/proto/datarightspb"
	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	financepb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
	sharedreporting "github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"google.golang.org/protobuf/proto"
)

type MetricResult struct {
	Name            string
	Values          *reportingpb.CountValues
	UnavailableCode reportingpb.MetricErrorCode
}

func (m MetricResult) IsAvailable() bool {
	return m.Values != nil
}

type GroupResult struct {
	Name      string
	WindowSet *reportingpb.ReportWindowSet
	Metrics   []MetricResult
	Err       error
}

type Collection struct {
	Groups []GroupResult
}

func (c Collection) Group(name string) (GroupResult, bool) {
	for _, group := range c.Groups {
		if group.Name == name {
			return group, true
		}
	}
	return GroupResult{}, false
}

type GroupClient interface {
	Name() string
	Collect(context.Context, *reportingpb.ReportWindowSet) GroupResult
}

type Collector struct {
	Clients []GroupClient
	Timeout time.Duration
}

func (c Collector) Collect(ctx context.Context, windows *reportingpb.ReportWindowSet) Collection {
	results := make(chan GroupResult, len(c.Clients))
	for _, client := range c.Clients {
		client := client
		go func() {
			requestContext := ctx
			cancel := func() {}
			if c.Timeout > 0 {
				requestContext, cancel = context.WithTimeout(ctx, c.Timeout)
			}
			defer cancel()
			result := client.Collect(requestContext, cloneWindowSet(windows))
			if result.Name == "" {
				result.Name = client.Name()
			}
			results <- result
		}()
	}

	collection := Collection{Groups: make([]GroupResult, 0, len(c.Clients))}
	for range c.Clients {
		collection.Groups = append(collection.Groups, <-results)
	}
	sort.SliceStable(collection.Groups, func(i, j int) bool {
		return collection.Groups[i].Name < collection.Groups[j].Name
	})
	return collection
}

func cloneWindowSet(windows *reportingpb.ReportWindowSet) *reportingpb.ReportWindowSet {
	if windows == nil {
		return nil
	}
	return proto.Clone(windows).(*reportingpb.ReportWindowSet)
}

type rpcGroup struct {
	name string
	call func(context.Context, *reportingpb.ReportWindowSet) GroupResult
}

func (g rpcGroup) Name() string { return g.name }
func (g rpcGroup) Collect(ctx context.Context, windows *reportingpb.ReportWindowSet) GroupResult {
	return g.call(ctx, windows)
}

func NewAuthGroup(client authpb.AuthServiceClient) GroupClient {
	return rpcGroup{name: "auth", call: func(ctx context.Context, windows *reportingpb.ReportWindowSet) GroupResult {
		response, err := client.GetGrowthMetrics(ctx, &authpb.GetGrowthMetricsRequest{WindowSet: windows})
		if err != nil {
			return failedGroup("auth", err)
		}
		if response == nil {
			return failedGroup("auth", errors.New("empty response"))
		}
		return buildGroup("auth", response.GetWindowSet(), []MetricResult{
			newMetric("new_users", response.GetNewUsers()),
			newMetric("onboarding_completions", response.GetOnboardingCompletions()),
		}, windows)
	}}
}

func NewExpenseGroup(client expensepb.ExpenseServiceClient) GroupClient {
	return rpcGroup{name: "expense", call: func(ctx context.Context, windows *reportingpb.ReportWindowSet) GroupResult {
		response, err := client.GetActivityMetrics(ctx, &expensepb.GetActivityMetricsRequest{WindowSet: windows})
		if err != nil {
			return failedGroup("expense", err)
		}
		if response == nil {
			return failedGroup("expense", errors.New("empty response"))
		}
		return buildGroup("expense", response.GetWindowSet(), []MetricResult{
			newMetric("total_expenses", response.GetTotalExpenses()),
			newMetric("manual_expenses", response.GetManualExpenses()),
			newMetric("correction_expenses", response.GetCorrectionExpenses()),
			newMetric("prorata_expenses", response.GetProrataExpenses()),
			newMetric("active_expenses", response.GetActiveExpenses()),
		}, windows)
	}}
}

func NewFinanceGroup(client financepb.FinanceServiceClient) GroupClient {
	return rpcGroup{name: "finance", call: func(ctx context.Context, windows *reportingpb.ReportWindowSet) GroupResult {
		response, err := client.GetProductUsageMetrics(ctx, &financepb.GetProductUsageMetricsRequest{WindowSet: windows})
		if err != nil {
			return failedGroup("finance", err)
		}
		if response == nil {
			return failedGroup("finance", errors.New("empty response"))
		}
		return buildGroup("finance", response.GetWindowSet(), []MetricResult{
			newMetric("budget_periods", response.GetBudgetPeriods()),
			newMetric("tags", response.GetTags()),
			newMetric("prorata_schedules", response.GetProrataSchedules()),
		}, windows)
	}}
}

func NewDatarightsGroup(client datarightspb.DatarightsServiceClient) GroupClient {
	return rpcGroup{name: "datarights", call: func(ctx context.Context, windows *reportingpb.ReportWindowSet) GroupResult {
		response, err := client.GetExportMetrics(ctx, &datarightspb.GetExportMetricsRequest{WindowSet: windows})
		if err != nil {
			return failedGroup("datarights", err)
		}
		if response == nil {
			return failedGroup("datarights", errors.New("empty response"))
		}
		return buildGroup("datarights", response.GetWindowSet(), []MetricResult{
			newMetric("completed_exports", response.GetCompletedExports()),
		}, windows)
	}}
}

func newMetric(name string, result *reportingpb.CountResult) MetricResult {
	if result == nil {
		return MetricResult{Name: name, UnavailableCode: sharedreporting.MetricErrorCodeInternal}
	}
	if values := result.GetAvailable(); values != nil {
		return MetricResult{Name: name, Values: values}
	}
	if unavailable := result.GetUnavailable(); unavailable != nil {
		return MetricResult{Name: name, UnavailableCode: unavailable.GetCode()}
	}
	return MetricResult{Name: name, UnavailableCode: sharedreporting.MetricErrorCodeInternal}
}

func buildGroup(name string, responseWindows *reportingpb.ReportWindowSet, metrics []MetricResult, requested *reportingpb.ReportWindowSet) GroupResult {
	result := GroupResult{Name: name, WindowSet: responseWindows, Metrics: metrics}
	if responseWindows == nil {
		result.Err = &WindowError{Group: name, Cause: errors.New("response window set is missing")}
		markWindowMetricsUnavailable(result.Metrics)
		return result
	}
	if err := sharedreporting.ValidateWindowSet(responseWindows); err != nil {
		result.Err = &WindowError{Group: name, Cause: err}
		markWindowMetricsUnavailable(result.Metrics)
		return result
	}
	if requested != nil && !proto.Equal(responseWindows, requested) {
		result.Err = &WindowError{Group: name, Cause: errors.New("response window set does not match request")}
		markWindowMetricsUnavailable(result.Metrics)
	}
	return result
}

func markWindowMetricsUnavailable(metrics []MetricResult) {
	for index := range metrics {
		metrics[index].Values = nil
		metrics[index].UnavailableCode = sharedreporting.MetricErrorCodeInvalidWindow
	}
}

func failedGroup(name string, err error) GroupResult {
	metrics := make([]MetricResult, 0, len(metricNames(name)))
	for _, metricName := range metricNames(name) {
		metrics = append(metrics, MetricResult{Name: metricName, UnavailableCode: sharedreporting.MetricErrorCodeQueryFailed})
	}
	return GroupResult{Name: name, Metrics: metrics, Err: &RPCError{Group: name, Cause: err}}
}

func metricNames(group string) []string {
	switch group {
	case "auth":
		return []string{"new_users", "onboarding_completions"}
	case "expense":
		return []string{"total_expenses", "manual_expenses", "correction_expenses", "prorata_expenses", "active_expenses"}
	case "finance":
		return []string{"budget_periods", "tags", "prorata_schedules"}
	case "datarights":
		return []string{"completed_exports"}
	default:
		return nil
	}
}

type RPCError struct {
	Group string
	Cause error
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s RPC failed", e.Group)
}

func (e *RPCError) Unwrap() error { return e.Cause }

type WindowError struct {
	Group string
	Cause error
}

func (e *WindowError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s returned an invalid reporting window", e.Group)
}

func (e *WindowError) Unwrap() error { return e.Cause }
