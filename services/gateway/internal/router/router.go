package router

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

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
	// canonical policy table and enforces Public/Authenticated/Personal/Admin,
	// replacing the former per-request auth + per-group admin guards.
	engine.Use(access.AccessControl(validator, access.DefaultPolicy(), logger))

	// Prometheus metrics endpoint (Public in the policy table).
	metrics.Register(engine)

	// Health check endpoint (Public in the policy table).
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Build reverse proxy handlers for each downstream service.
	authProxy := proxy.NewServiceProxy(serviceURLs.AuthREST, logger)
	expenseProxy := proxy.NewServiceProxy(serviceURLs.ExpenseREST, logger)
	financeProxy := proxy.NewServiceProxy(serviceURLs.FinanceREST, logger)
	datarightsProxy := proxy.NewServiceProxy(serviceURLs.DatarightsREST, logger)

	// Route groups are pure proxy wiring: access is enforced globally by
	// AccessControl against the policy table, not per group.

	// /api/auth/* → Auth service (REST)
	authGroup := engine.Group("/api/auth")
	{
		authGroup.Any("", ginWrapHandler(authProxy))
		authGroup.Any("/*path", ginWrapHandler(authProxy))
	}

	// /api/admin/* → Auth service (REST)
	adminGroup := engine.Group("/api/admin")
	{
		adminGroup.Any("", ginWrapHandler(authProxy))
		adminGroup.Any("/*path", ginWrapHandler(authProxy))
	}

	// /api/expenses/* → Expense service (REST)
	expenseGroup := engine.Group("/api/expenses")
	{
		expenseGroup.Any("", ginWrapHandler(expenseProxy))
		expenseGroup.Any("/*path", ginWrapHandler(expenseProxy))
	}

	// /api/finance/* → Finance service (REST)
	financeGroup := engine.Group("/api/finance")
	{
		financeGroup.Any("", ginWrapHandler(financeProxy))
		financeGroup.Any("/*path", ginWrapHandler(financeProxy))
	}

	// /api/datarights/* → Datarights service (REST)
	datarightsGroup := engine.Group("/api/datarights")
	{
		datarightsGroup.Any("", ginWrapHandler(datarightsProxy))
		datarightsGroup.Any("/*path", ginWrapHandler(datarightsProxy))
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
