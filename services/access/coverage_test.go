package access

import (
	"strings"
	"testing"
)

// registeredFromRegistry returns the RegisteredRoute set a correctly wired
// service would produce: exactly its Registry entries.
func registeredFromRegistry(service string) []RegisteredRoute {
	routes := RoutesFor(service)
	registered := make([]RegisteredRoute, 0, len(routes))
	for _, r := range routes {
		registered = append(registered, RegisteredRoute{Method: r.Method, Path: r.Path})
	}
	return registered
}

// TestVerifyRegistration_MatchingRouterPasses confirms a router that registers
// exactly the Registry entries for a service (plus ignored health/metrics)
// verifies clean.
func TestVerifyRegistration_MatchingRouterPasses(t *testing.T) {
	for _, service := range []string{"auth", "finance", "expense", "datarights"} {
		registered := append(registeredFromRegistry(service),
			RegisteredRoute{Method: "GET", Path: "/health"},
			RegisteredRoute{Method: "GET", Path: "/metrics"},
		)
		if err := VerifyRegistration(service, registered); err != nil {
			t.Errorf("VerifyRegistration(%q) = %v, want nil", service, err)
		}
	}
}

// TestVerifyRegistration_ExtraRouteIsReported is direction 1: an /api route
// with no Registry entry is flagged and named.
func TestVerifyRegistration_ExtraRouteIsReported(t *testing.T) {
	registered := append(registeredFromRegistry("auth"),
		RegisteredRoute{Method: "GET", Path: "/api/auth/secret"},
	)

	err := VerifyRegistration("auth", registered)
	if err == nil {
		t.Fatal("expected an error for a route missing from the Registry")
	}
	if !strings.Contains(err.Error(), "GET /api/auth/secret") {
		t.Errorf("error should name the offending route, got: %v", err)
	}
}

// TestVerifyRegistration_MissingRouteIsReported is direction 2: a Registry
// entry the service failed to register is flagged and named.
func TestVerifyRegistration_MissingRouteIsReported(t *testing.T) {
	registered := registeredFromRegistry("auth")
	// Drop the last route to simulate a forgotten binding.
	registered = registered[:len(registered)-1]

	err := VerifyRegistration("auth", registered)
	if err == nil {
		t.Fatal("expected an error for an unregistered Registry route")
	}
	if !strings.Contains(err.Error(), "GET /api/admin/users") {
		t.Errorf("error should name the missing route, got: %v", err)
	}
}
