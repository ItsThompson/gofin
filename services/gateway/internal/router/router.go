package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
	"github.com/ItsThompson/gofin/services/gateway/internal/proxy"
	"github.com/ItsThompson/gofin/services/gateway/internal/readiness"
	"github.com/ItsThompson/gofin/services/metrics"
)

// readinessTimeout bounds each downstream /health probe issued by /readyz. It
// mirrors the healthcheck lib's 2s Docker HEALTHCHECK timeout so /readyz and
// container liveness share the same notion of "slow enough to be down".
const readinessTimeout = 2 * time.Second

// ServiceURLs holds the parsed downstream service URLs.
type ServiceURLs struct {
	AuthREST       *url.URL
	ExpenseREST    *url.URL
	FinanceREST    *url.URL
	DatarightsREST *url.URL
}

// New creates a configured Gin engine with all gateway routes, middleware,
// and reverse proxy handlers wired up. The proxy groups are derived from the
// injected prefixes (the shared services/access inventory, obtained via
// sharedaccess.Prefixes()); injecting them keeps New a pure function of its
// inputs and lets tests construct a wiring without mutating shared state.
func New(
	validator access.TokenValidator,
	serviceURLs *ServiceURLs,
	prefixes []sharedaccess.ProxyPrefix,
	logger *slog.Logger,
	isProduction bool,
) *gin.Engine {
	if isProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.Use(gin.Recovery())
	engine.Use(metrics.HTTPMetrics())
	engine.Use(middleware.RequestLogger(logger))
	// AccessControl is the single global gate: it resolves each route against the
	// shared services/access registry (via GatewayResolve, which also classifies
	// the gateway-native /health and /metrics as Public) and enforces
	// Public/Authenticated/Personal/Admin, replacing the former per-request auth +
	// per-group admin guards.
	engine.Use(access.AccessControl(validator, access.GatewayResolve, logger))

	// Prometheus metrics endpoint (Public via GatewayResolve).
	metrics.Register(engine)

	// Health check endpoint (Public via GatewayResolve).
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Readiness aggregate (Public via GatewayResolve). Unlike the shallow
	// /health above, /readyz fans out to every downstream service's /health so a
	// single probe proves the whole backend is reachable. The service-name keys
	// are the same canonical names used for the proxy handlers below, so the 503
	// body and logs name the service the gateway proxies to.
	checker := readiness.NewChecker(&http.Client{}, servicesFromURLs(serviceURLs), readinessTimeout)
	engine.GET("/readyz", readiness.Handler(checker, logger))

	// Build one reverse-proxy handler per downstream service, keyed by the
	// service name used in the shared Registry and ProxyPrefixes.
	proxies := map[string]http.Handler{
		"auth":       proxy.NewServiceProxy(serviceURLs.AuthREST, logger),
		"expense":    proxy.NewServiceProxy(serviceURLs.ExpenseREST, logger),
		"finance":    proxy.NewServiceProxy(serviceURLs.FinanceREST, logger),
		"datarights": proxy.NewServiceProxy(serviceURLs.DatarightsREST, logger),
	}

	// Derive the proxy wiring from the injected prefix inventory so onboarding a
	// service is a single edit to services/access.proxyPrefixes (which the
	// cross-check test pins to the Registry). Access is enforced globally by
	// AccessControl against the Registry, not per group. Fail fast if a prefix
	// names a service with no proxy handler.
	for _, p := range prefixes {
		handler, ok := proxies[p.Service]
		if !ok {
			panic(fmt.Sprintf(
				"ProxyPrefix %q names service %q, which has no proxy handler; add its ServiceURL and proxy to router.New",
				p.Prefix, p.Service,
			))
		}
		group := engine.Group(p.Prefix)
		group.Any("", ginWrapHandler(handler))
		group.Any("/*path", ginWrapHandler(handler))
	}

	return engine
}

// ginWrapHandler adapts a standard http.Handler to a Gin handler function.
// This lets us use httputil.ReverseProxy (which implements http.Handler)
// inside Gin route groups.
func ginWrapHandler(handler http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// servicesFromURLs builds the canonical service-name -> base URL map the
// readiness checker fans out over, keyed by the same names as the proxy
// handlers so /readyz reports the service the gateway actually proxies to.
func servicesFromURLs(serviceURLs *ServiceURLs) map[string]string {
	return map[string]string{
		"auth":       serviceURLs.AuthREST.String(),
		"expense":    serviceURLs.ExpenseREST.String(),
		"finance":    serviceURLs.FinanceREST.String(),
		"datarights": serviceURLs.DatarightsREST.String(),
	}
}
