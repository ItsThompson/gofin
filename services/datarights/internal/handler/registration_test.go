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
// Registry in both directions. datarights binds handlers from both the export
// and deletion handlers; adding an unclassified route, or a Registry entry with
// no handler, fails here in datarights' own module (run by CI), pointing at the
// Registry.
func TestRegisterRoutes_MatchesRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	engine := gin.New()
	RegisterRoutes(engine, NewRESTHandler(nil, logger), NewDeletionHandler(nil, logger))

	registered := make([]access.RegisteredRoute, 0)
	for _, r := range engine.Routes() {
		registered = append(registered, access.RegisteredRoute{Method: r.Method, Path: r.Path})
	}

	if err := access.VerifyRegistration("datarights", registered); err != nil {
		t.Fatal(err)
	}
}
