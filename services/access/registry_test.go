package access

import "testing"

// TestRegistry_IDsAreUnique guards the bind-by-ID contract: services look up
// handlers by Route.ID, so a duplicate ID would silently shadow a handler.
func TestRegistry_IDsAreUnique(t *testing.T) {
	seen := make(map[string]Route, len(Registry))
	for _, r := range Registry {
		if prev, dup := seen[r.ID]; dup {
			t.Errorf("duplicate route ID %q: %s %s and %s %s", r.ID, prev.Method, prev.Path, r.Method, r.Path)
		}
		seen[r.ID] = r
	}
}

// TestRegistry_EveryEntryResolvesToItsAccess derives the acceptance assertion
// directly from the Registry (no second hand-list): every entry's own
// method+path must resolve to the Access it declares.
func TestRegistry_EveryEntryResolvesToItsAccess(t *testing.T) {
	for _, r := range Registry {
		if got := Resolve(r.Method, r.Path); got != r.Access {
			t.Errorf("Resolve(%q, %q) = %s, want %s (route %s)", r.Method, r.Path, got, r.Access, r.ID)
		}
	}
}

// TestRegistry_EveryEntryIsClassified confirms Classifies agrees with the
// Registry for every real route: each entry's path is matched by an explicit
// entry rather than the fail-safe default.
func TestRegistry_EveryEntryIsClassified(t *testing.T) {
	for _, r := range Registry {
		if !Classifies(r.Method, r.Path) {
			t.Errorf("Classifies(%q, %q) = false, want true (route %s)", r.Method, r.Path, r.ID)
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

	if total != len(Registry) {
		t.Errorf("RoutesFor over %v covered %d routes, but Registry has %d; a service is misnamed or missing", services, total, len(Registry))
	}
}

// TestRoutesFor_UnknownServiceIsEmpty confirms an unknown service yields no
// routes rather than panicking or leaking cross-service entries.
func TestRoutesFor_UnknownServiceIsEmpty(t *testing.T) {
	if routes := RoutesFor("nope"); len(routes) != 0 {
		t.Errorf("RoutesFor(%q) = %d routes, want 0", "nope", len(routes))
	}
}

// TestClassifies_UnknownRoutesAreUnclassified is the failure-mode guardrail: a
// route with no Registry entry, or a real path under the wrong method, is not
// classified, so Resolve returns the fail-safe Deny default (403 at the
// gateway).
func TestClassifies_UnknownRoutesAreUnclassified(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"unknown service", "GET", "/api/newservice/records"},
		{"wrong method on a real route", "GET", "/api/auth/login"},
		{"sibling substring of a real route", "POST", "/api/datarights/exports-admin"},
		{"bare group with no exact route", "GET", "/api/finance"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if Classifies(tc.method, tc.path) {
				t.Errorf("Classifies(%q, %q) = true, want false", tc.method, tc.path)
			}
			if got := Resolve(tc.method, tc.path); got != Deny {
				t.Errorf("Resolve(%q, %q) = %s, want Deny (fail-safe default)", tc.method, tc.path, got)
			}
		})
	}
}
