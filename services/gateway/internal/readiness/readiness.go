// Package readiness aggregates the liveness of the downstream services behind
// the gateway. It backs the gateway-native GET /readyz endpoint, which reports
// 200 only when every downstream service's shallow /health returns 200, and 503
// (naming the unhealthy/unreachable services) otherwise.
//
// It is deliberately separate from the gateway's own /health: /health is the
// shallow container-liveness signal wired to Docker HEALTHCHECK, whereas /readyz
// fans out to the downstream services so a single probe can prove the whole
// backend is reachable. Coupling the two would risk container restart loops on a
// transient downstream blip.
package readiness

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Per-service status values reported in a Result. A concrete non-200 status is
// reported as "status_<code>" (e.g. "status_503") so the body distinguishes a
// service that answered unhealthily from one that could not be reached at all.
const (
	statusOK          = "ok"
	statusUnreachable = "unreachable"
)

// Checker probes a fixed set of downstream services by issuing a concurrent GET
// {baseURL}/health to each, bounded by a per-probe timeout. It is a deep module:
// callers build it once with NewChecker and call Check; the fan-out, timeout,
// and result classification are hidden behind a single method.
type Checker struct {
	client   *http.Client
	services map[string]string // service name -> base URL
	timeout  time.Duration
}

// NewChecker builds a Checker over the given service name -> base URL map. The
// keys are the canonical service names (auth|expense|finance|datarights) so the
// 503 body and logs name the same service the gateway proxies to. The injected
// client and timeout keep Check a pure function of its inputs and testable
// against httptest downstreams.
func NewChecker(client *http.Client, services map[string]string, timeout time.Duration) *Checker {
	return &Checker{client: client, services: services, timeout: timeout}
}

// Result is the aggregate outcome of a readiness probe. Services maps each
// service name to its status ("ok" | "unreachable" | "status_<code>").
type Result struct {
	Healthy  bool
	Services map[string]string
}

// Check probes every configured service concurrently and returns once all
// probes settle. Healthy is true only when every service's /health returned
// 200. The fan-out is concurrent so total latency is bounded by the slowest
// single probe (the per-probe timeout), not their sum.
func (c *Checker) Check(ctx context.Context) Result {
	type probeResult struct {
		name   string
		status string
		ok     bool
	}

	results := make(chan probeResult, len(c.services))
	var waitGroup sync.WaitGroup
	for name, baseURL := range c.services {
		waitGroup.Add(1)
		go func(name, baseURL string) {
			defer waitGroup.Done()
			status, ok := c.probe(ctx, baseURL)
			results <- probeResult{name: name, status: status, ok: ok}
		}(name, baseURL)
	}
	waitGroup.Wait()
	close(results)

	statuses := make(map[string]string, len(c.services))
	healthy := true
	for result := range results {
		statuses[result.name] = result.status
		if !result.ok {
			healthy = false
		}
	}
	return Result{Healthy: healthy, Services: statuses}
}

// probe issues a single GET {baseURL}/health bounded by the checker's timeout.
// A transport error or an unbuildable request is "unreachable"; any non-200
// response is reported as "status_<code>".
func (c *Checker) probe(ctx context.Context, baseURL string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return statusUnreachable, false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return statusUnreachable, false
	}
	// Drain then close so the underlying connection can be reused (keep-alive).
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		return statusOK, true
	}
	return fmt.Sprintf("status_%d", resp.StatusCode), false
}

// Handler adapts a Checker into the gin handler for GET /readyz: 200
// {"status":"ok"} when every downstream is healthy, else 503
// {"status":"unhealthy","services":{...}} naming each service's state. On a 503
// it emits a structured warning naming the per-service statuses so the failure
// is diagnosable from logs alone.
func Handler(checker *Checker, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := checker.Check(c.Request.Context())
		if result.Healthy {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}

		logger.Warn("readiness check failed",
			slog.Any("services", result.Services),
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unhealthy",
			"services": result.Services,
		})
	}
}
