// Package healthcheck provides a binary self-healthcheck for Go services.
// When a service binary is invoked with --healthcheck, it performs an HTTP GET
// to localhost on the service's health endpoint and exits 0 (healthy) or 1
// (unhealthy/unreachable). This enables Docker HEALTHCHECK without requiring
// wget, curl, or a shell in the container image.
package healthcheck

import (
	"fmt"
	"net/http"
	"time"
)

// Run performs an HTTP GET to http://localhost:{port}/health.
// Returns 0 if the response status is 200 OK, 1 otherwise.
func Run(port string) int {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", port))
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	return 1
}

// ShouldRun returns true if the process arguments contain "--healthcheck".
// Call this at the top of main() before service initialization.
func ShouldRun(args []string) bool {
	return len(args) > 1 && args[1] == "--healthcheck"
}
