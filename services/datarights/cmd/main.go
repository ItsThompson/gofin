package main

import (
	"fmt"
	"os"

	"github.com/ItsThompson/gofin/services/datarights/internal/bootstrap"
	"github.com/ItsThompson/gofin/services/datarights/internal/config"
	"github.com/ItsThompson/gofin/services/healthcheck"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run(config.RESTPort()))
	}

	if err := bootstrap.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "datarights-service: %v\n", err)
		os.Exit(1)
	}
}
