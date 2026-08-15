package access

import (
	"net/http"
	"testing"
)

// TestRegistry_IDsAreUnique guards the bind-by-ID contract: services look up
// handlers by Route.ID, so a duplicate ID would silently shadow a handler.
func TestRegistry_IDsAreUnique(t *testing.T) {
	seen := make(map[string]Route, len(registry))
	for _, r := range registry {
		if prev, dup := seen[r.ID]; dup {
			t.Errorf("duplicate route ID %q: %s %s and %s %s", r.ID, prev.Method, prev.Path, r.Method, r.Path)
		}
		seen[r.ID] = r
	}
}

// TestRegistry_EveryEntryResolvesToItsAccess derives the assertion
// directly from the Registry (no second hand-list): every entry's own
// method+path must resolve to the Access it declares.
func TestRegistry_EveryEntryResolvesToItsAccess(t *testing.T) {
	for _, r := range registry {
		if got := Resolve(r.Method, r.Path); got != r.Access {
			t.Errorf("Resolve(%q, %q) = %s, want %s (route %s)", r.Method, r.Path, got, r.Access, r.ID)
		}
	}
}

// TestRoutesFor_PartitionsRegistry proves RoutesFor slices the Registry cleanly:
// the four known services together account for every entry with no gaps or
// duplicates, and each returned route actually belongs to the requested service.
func TestRoutesFor_PartitionsRegistry(t *testing.T) {
	services := []string{"auth", "finance", "expense", "datarights"}

	total := 0
	for _, svc := range services {
		routes := RoutesFor(svc)
		for _, r := range routes {
			if r.Service != svc {
				t.Errorf("RoutesFor(%q) returned route %s with Service %q", svc, r.ID, r.Service)
			}
		}
		total += len(routes)
	}

	if total != len(registry) {
		t.Errorf("RoutesFor over %v covered %d routes, but Registry has %d; a service is misnamed or missing", services, total, len(registry))
	}
}

// TestRoutesFor_UnknownServiceIsEmpty confirms an unknown service yields no
// routes rather than panicking or leaking cross-service entries.
func TestRoutesFor_UnknownServiceIsEmpty(t *testing.T) {
	for _, service := range []string{"nope", "fx"} {
		if routes := RoutesFor(service); len(routes) != 0 {
			t.Errorf("RoutesFor(%q) = %d routes, want 0", service, len(routes))
		}
	}
}

// TestRouteID_EveryEntryResolvesToItsOwnID derives the assertion from the
// Registry rather than a second hand-list, the same way the Access test does.
func TestRouteID_EveryEntryResolvesToItsOwnID(t *testing.T) {
	for _, r := range registry {
		if got := RouteID(r.Method, r.Path); got != r.ID {
			t.Errorf("RouteID(%q, %q) = %q, want %q", r.Method, r.Path, got, r.ID)
		}
	}
}

// TestRouteID_NoMatchIsEmpty covers what a reporter sees for an unregistered
// route: the empty pattern a handler running outside the router produces, a path
// no Registry entry declares, and the right path under the wrong method.
func TestRouteID_NoMatchIsEmpty(t *testing.T) {
	cases := []struct{ method, pattern string }{
		{http.MethodGet, ""},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/api/fx"},
		{http.MethodDelete, "/api/expenses"},
	}

	for _, tc := range cases {
		if got := RouteID(tc.method, tc.pattern); got != "" {
			t.Errorf("RouteID(%q, %q) = %q, want \"\"", tc.method, tc.pattern, got)
		}
	}
}
