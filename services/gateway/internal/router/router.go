package router

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
	"github.com/ItsThompson/gofin/services/gateway/internal/proxy"
	"github.com/ItsThompson/gofin/services/metrics"
)

// ServiceURLs holds the parsed downstream service URLs.
type ServiceURLs struct {
	AuthREST    *url.URL
	ExpenseREST *url.URL
	FinanceREST *url.URL
}

// New creates a configured Gin engine with all gateway routes, middleware,
// and reverse proxy handlers wired up.
func New(
	validator middleware.TokenValidator,
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
	engine.Use(middleware.Auth(validator, logger))

	// Prometheus metrics endpoint (excluded from auth middleware via exception list).
	metrics.Register(engine)

	// Health check endpoint: auth middleware skips it via the exception list.
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Build reverse proxy handlers for each downstream service.
	authProxy := proxy.NewServiceProxy(serviceURLs.AuthREST, logger)
	expenseProxy := proxy.NewServiceProxy(serviceURLs.ExpenseREST, logger)
	financeProxy := proxy.NewServiceProxy(serviceURLs.FinanceREST, logger)

	// /api/auth/* → Auth service (REST)
	// Some auth routes are unauthenticated (register, login, refresh) and
	// bypass the auth middleware via the exception list in auth.go.
	// POST /api/auth/assume requires admin role via AdminRouteGuard.
	authGroup := engine.Group("/api/auth")
	authGroup.Use(middleware.AdminRouteGuard(logger))
	{
		authGroup.Any("", ginWrapHandler(authProxy))
		authGroup.Any("/*path", ginWrapHandler(authProxy))
	}

	// /api/admin/* → Auth service (REST), admin-only
	adminGroup := engine.Group("/api/admin")
	adminGroup.Use(middleware.RequireAdmin(logger))
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
