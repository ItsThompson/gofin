package handler

import (
	"io"
	"log/slog"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/access"
)

// TestRegisterRoutes_MatchesRegistry builds the real engine via the registry-
// driven RegisterRoutes with nil service deps (gin does not execute handlers at
// registration) and asserts the registered routes match the services/access
// Registry in both directions. Adding an unclassified route, or a Registry
// entry with no handler, fails here in finance's own module (run by CI),
// pointing at the Registry.
func TestRegisterRoutes_MatchesRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	engine := gin.New()
	NewRESTHandler(nil, logger).RegisterRoutes(engine)

	registered := make([]access.RegisteredRoute, 0)
	for _, r := range engine.Routes() {
		registered = append(registered, access.RegisteredRoute{Method: r.Method, Path: r.Path})
	}

	if err := access.VerifyRegistration("finance", registered); err != nil {
		t.Fatal(err)
	}
}
