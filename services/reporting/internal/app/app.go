package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ItsThompson/gofin/services/reporting/internal/aggregator"
	"github.com/ItsThompson/gofin/services/reporting/internal/collector"
	"github.com/ItsThompson/gofin/services/reporting/internal/config"
	"github.com/ItsThompson/gofin/services/reporting/internal/formatter"
)

type Sender interface {
	Send(context.Context, string) error
}

type Dependencies struct {
	Groups []collector.GroupClient
	Sender Sender
	Output io.Writer
	Now    func() time.Time
}

type Result struct {
	Report    aggregator.Report
	Text      string
	Partial   bool
	Delivered bool
}

func Run(ctx context.Context, reportDate string, dryRun bool, cfg config.Config, deps Dependencies) (Result, error) {
	if cfg.RPCTimeout <= 0 {
		cfg.RPCTimeout = config.DefaultRPCTimeout
	}
	if cfg.CollectionTimeout <= 0 {
		cfg.CollectionTimeout = config.DefaultCollectionTimeout
	}
	if cfg.DeliveryTimeout <= 0 {
		cfg.DeliveryTimeout = config.DefaultDeliveryTimeout
	}
	if cfg.AppTimeout <= 0 {
		cfg.AppTimeout = config.DefaultAppTimeout
	}
	now := time.Now
	if deps.Now != nil {
		now = deps.Now
	}
	generatedAt := now().UTC()
	start, err := config.ResolveReportDate(reportDate, generatedAt)
	if err != nil {
		return Result{}, &LifecycleError{Stage: "date", Cause: err}
	}
	windows, err := aggregator.BuildWindowSet(start)
	if err != nil {
		return Result{}, &LifecycleError{Stage: "window", Cause: err}
	}
	if len(deps.Groups) != 4 {
		return Result{}, &LifecycleError{Stage: "clients", Cause: fmt.Errorf("expected four reporting clients")}
	}
	runContext := ctx
	cancel := func() {}
	if cfg.AppTimeout > 0 {
		runContext, cancel = context.WithTimeout(ctx, cfg.AppTimeout)
	}
	defer cancel()
	collectionContext := runContext
	cancelCollection := func() {}
	if cfg.CollectionTimeout > 0 {
		collectionContext, cancelCollection = context.WithTimeout(runContext, cfg.CollectionTimeout)
	}
	collection := (collector.Collector{Clients: deps.Groups, Timeout: cfg.RPCTimeout}).Collect(collectionContext, windows)
	cancelCollection()
	report := aggregator.Aggregate(collection, windows)
	report.GeneratedAt = generatedAt
	text := formatter.Format(report)
	output := deps.Output
	if output == nil {
		output = io.Discard
	}
	if _, err := io.WriteString(output, text); err != nil {
		return Result{Report: report, Text: text, Partial: hasGroupFailure(report)}, &LifecycleError{Stage: "output", Cause: err}
	}
	result := Result{Report: report, Text: text, Partial: hasGroupFailure(report)}
	if err := formatter.ValidateRuneLimit(text); err != nil {
		return result, &LifecycleError{Stage: "size", Cause: err}
	}
	if runContext.Err() != nil {
		return result, &LifecycleError{Stage: "context", Cause: runContext.Err()}
	}
	if !dryRun {
		if deps.Sender == nil {
			return result, &LifecycleError{Stage: "discord", Cause: fmt.Errorf("discord sender is required")}
		}
		deliveryContext := runContext
		cancelDelivery := func() {}
		if cfg.DeliveryTimeout > 0 {
			deliveryContext, cancelDelivery = context.WithTimeout(runContext, cfg.DeliveryTimeout)
		}
		err := deps.Sender.Send(deliveryContext, text)
		cancelDelivery()
		if err != nil {
			if runContext.Err() != nil {
				return result, &LifecycleError{Stage: "context", Cause: runContext.Err()}
			}
			return result, &LifecycleError{Stage: "discord", Cause: err}
		}
		result.Delivered = true
	}
	if result.Partial {
		return result, &PartialError{Groups: failedGroups(report)}
	}
	return result, nil
}

func hasGroupFailure(report aggregator.Report) bool {
	return len(failedGroups(report)) > 0
}

func failedGroups(report aggregator.Report) []string {
	groups := make([]string, 0)
	for _, group := range report.Groups {
		failed := group.Err != nil
		for _, metric := range group.Metrics {
			if !metric.Available {
				failed = true
				break
			}
		}
		if failed {
			groups = append(groups, group.Name)
		}
	}
	return groups
}

type LifecycleError struct {
	Stage string
	Cause error
}

func (e *LifecycleError) Error() string {
	if e == nil {
		return ""
	}
	return e.Stage + " stage failed"
}

func (e *LifecycleError) Unwrap() error { return e.Cause }

type PartialError struct {
	Groups []string
}

func (e *PartialError) Error() string {
	if e == nil {
		return ""
	}
	return "reporting groups failed: " + strings.Join(e.Groups, ", ")
}
