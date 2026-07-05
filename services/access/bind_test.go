package access

import (
	"strings"
	"testing"
)

// TestBindRoutes_RegistersEveryRouteForService confirms BindRoutes hands every
// Registry route for a service to register exactly once, in Registry order,
// looked up by ID.
func TestBindRoutes_RegistersEveryRouteForService(t *testing.T) {
	want := RoutesFor("expense")

	handlers := make(map[string]int, len(want))
	for _, r := range want {
		handlers[r.ID] = 1
	}

	var registered []string
	BindRoutes("expense", handlers, func(method, path string, _ int) {
		registered = append(registered, method+" "+path)
	})

	if len(registered) != len(want) {
		t.Fatalf("registered %d routes, want %d", len(registered), len(want))
	}
	for i, r := range want {
		if got := r.Method + " " + r.Path; registered[i] != got {
			t.Errorf("registered[%d] = %q, want %q (order must follow Registry)", i, registered[i], got)
		}
	}
}

// TestBindRoutes_PanicsOnMissingHandler is the fail-fast guarantee: a Registry
// route with no handler in the map panics with a message naming the route, so a
// classified-but-unbound route is caught at startup rather than silently
// dropped.
func TestBindRoutes_PanicsOnMissingHandler(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for a route with no handler")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "auth.register") {
			t.Errorf("panic message = %v, want it to name the unbound route auth.register", r)
		}
	}()

	// An empty handler map cannot cover any auth route, so the first one panics.
	BindRoutes("auth", map[string]int{}, func(_, _ string, _ int) {})
}
