package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ItsThompson/gofin/services/reporting/internal/app"
	"github.com/ItsThompson/gofin/services/reporting/internal/clients"
	"github.com/ItsThompson/gofin/services/reporting/internal/collector"
	"github.com/ItsThompson/gofin/services/reporting/internal/config"
	"github.com/ItsThompson/gofin/services/reporting/internal/discord"
)

const (
	ExitSuccess  = 0
	ExitFailure  = 1
	ExitUsage    = 2
	ExitConfig   = 3
	ExitClients  = 4
	ExitReport   = 5
	ExitDiscord  = 6
	ExitPartial  = 7
	ExitCanceled = 130
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(parent context.Context, args []string, output, errorOutput io.Writer) int {
	flags := flag.NewFlagSet("reporting", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	reportWeekStart := flags.String("report-week-start", "", "report week start in YYYY-MM-DD format")
	dryRun := flags.Bool("dry-run", false, "print the report without delivering it")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(errorOutput, "reporting: invalid command-line arguments")
		return ExitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errorOutput, "reporting: unexpected command-line argument")
		return ExitUsage
	}
	if _, err := app.ResolveWindows(*reportWeekStart, time.Now().UTC()); err != nil {
		fmt.Fprintln(errorOutput, "reporting: invalid report week")
		return ExitUsage
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(errorOutput, "reporting: configuration failed")
		return ExitConfig
	}
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if ctx.Err() != nil {
		fmt.Fprintln(errorOutput, "reporting: canceled")
		return ExitCanceled
	}

	clientSet, err := clients.Dial(ctx, cfg)
	if err != nil {
		fmt.Fprintln(errorOutput, "reporting: client setup failed")
		return classifyError(err)
	}

	var sender app.Sender
	if !*dryRun {
		sender, err = discord.NewSender(cfg.DiscordWebhookURL, cfg.DiscordTimeout)
		if err != nil {
			_ = clientSet.Close()
			fmt.Fprintln(errorOutput, "reporting: Discord configuration failed")
			return ExitDiscord
		}
	}

	_, runErr := app.Run(ctx, *reportWeekStart, *dryRun, cfg, app.Dependencies{
		Groups: []collector.GroupClient{
			collector.NewAuthGroup(clientSet.Auth),
			collector.NewExpenseGroup(clientSet.Expense),
			collector.NewFinanceGroup(clientSet.Finance),
			collector.NewDatarightsGroup(clientSet.Datarights),
		},
		Sender: sender,
		Output: output,
	})
	closeErr := clientSet.Close()
	if runErr != nil {
		fmt.Fprintln(errorOutput, "reporting: "+stableError(runErr))
		return classifyError(runErr)
	}
	if closeErr != nil {
		fmt.Fprintln(errorOutput, "reporting: client shutdown failed")
		return ExitClients
	}
	return ExitSuccess
}

func stableError(err error) string {
	var lifecycle *app.LifecycleError
	if errors.As(err, &lifecycle) {
		return lifecycle.Stage + " stage failed"
	}
	var partial *app.PartialError
	if errors.As(err, &partial) {
		return partial.Error()
	}
	var dial *clients.DialError
	if errors.As(err, &dial) {
		return dial.Error()
	}
	return "operation failed"
}

func classifyError(err error) int {
	var lifecycle *app.LifecycleError
	if errors.As(err, &lifecycle) {
		switch lifecycle.Stage {
		case "date", "window":
			return ExitUsage
		case "clients":
			return ExitClients
		case "size", "output":
			return ExitReport
		case "discord":
			return ExitDiscord
		case "context":
			return ExitCanceled
		}
	}
	var dial *clients.DialError
	if errors.As(err, &dial) {
		return ExitClients
	}
	var partial *app.PartialError
	if errors.As(err, &partial) {
		return ExitPartial
	}
	return ExitFailure
}
