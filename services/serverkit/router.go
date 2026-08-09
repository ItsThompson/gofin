package serverkit

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/metrics"
)

// NewRouter builds a gin.Engine preloaded with Recovery, the shared HTTP
// metrics middleware, the /metrics endpoint, and a GET /health handler that
// reports the service name. Release mode is enabled when isProduction is true.
//
// This is the router for the four API services. The gateway keeps its own
// reverse-proxy router (router.New) and does not use NewRouter.
//
// The recovery records panics through slog.Default(), so callers install their
// logger (slog.SetDefault) before building the router.
func NewRouter(service string, isProduction bool) *gin.Engine {
	if isProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	// Recovery sits outside the metrics middleware so a panic raised there is
	// caught too.
	router.Use(Recovery(slog.Default()))
	router.Use(metrics.HTTPMetrics())
	metrics.Register(router)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": service})
	})

	return router
}
