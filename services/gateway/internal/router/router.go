package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
	"github.com/ItsThompson/gofin/services/gateway/internal/proxy"
	"github.com/ItsThompson/gofin/services/metrics"
)

// ServiceURLs holds the parsed downstream service URLs.
type ServiceURLs struct {
	AuthREST       *url.URL
	ExpenseREST    *url.URL
	FinanceREST    *url.URL
	DatarightsREST *url.URL
}

// New creates a configured Gin engine with all gateway routes, middleware,
// and reverse proxy handlers wired up.
func New(
	validator access.TokenValidator,
	serviceURLs *ServiceURLs,
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

	// Build one reverse-proxy handler per downstream service, keyed by the
	// service name used in the shared Registry and ProxyPrefixes.
	proxies := map[string]http.Handler{
		"auth":       proxy.NewServiceProxy(serviceURLs.AuthREST, logger),
		"expense":    proxy.NewServiceProxy(serviceURLs.ExpenseREST, logger),
		"finance":    proxy.NewServiceProxy(serviceURLs.FinanceREST, logger),
		"datarights": proxy.NewServiceProxy(serviceURLs.DatarightsREST, logger),
	}

	// Derive the proxy wiring from the shared prefix inventory so onboarding a
	// service is a single edit to services/access.ProxyPrefixes (which the
	// cross-check test pins to the Registry). Access is enforced globally by
	// AccessControl against the Registry, not per group. Fail fast if a prefix
	// names a service with no proxy handler.
	for _, p := range sharedaccess.ProxyPrefixes {
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
