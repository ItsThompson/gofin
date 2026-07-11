package serverkit

import (
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
func NewRouter(service string, isProduction bool) *gin.Engine {
	if isProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(metrics.HTTPMetrics())
	metrics.Register(router)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": service})
	})

	return router
}
