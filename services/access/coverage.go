package access

import (
	"fmt"
	"sort"
	"strings"
)

// RegisteredRoute is a concrete method+path a service actually registered on
// its router, typically collected from gin's engine.Routes(). Services hand a
// slice of these to VerifyRegistration to prove their router matches the
// Registry.
type RegisteredRoute struct {
	Method string
	Path   string
}

// VerifyRegistration compares a service's registered routes against the
// Registry in both directions and returns a descriptive error (naming the
// offending routes and pointing at the Registry) when they diverge:
//
//  1. every registered /api route corresponds byte-for-byte to a Registry entry
//     for this service, so no ad-hoc route escaped classification, and
//  2. every Registry entry for this service was registered, so no classified
//     route was left unbound.
//
// Only /api routes are considered, so a caller that also registers health or
// metrics endpoints is not penalized. Paths are compared exactly (including
// ":id"/":groupId" params), which is what pins the Registry patterns to gin's
// real registration.
func VerifyRegistration(service string, registered []RegisteredRoute) error {
	want := RoutesFor(service)

	wanted := make(map[RegisteredRoute]bool, len(want))
	for _, r := range want {
		wanted[RegisteredRoute{Method: r.Method, Path: r.Path}] = true
	}

	registeredSet := make(map[RegisteredRoute]bool, len(registered))
	var extra []string
	for _, r := range registered {
		registeredSet[r] = true
		if !strings.HasPrefix(r.Path, "/api/") {
			continue
		}
		if !wanted[r] {
			extra = append(extra, r.Method+" "+r.Path)
		}
	}

	var missing []string
	for _, r := range want {
		if !registeredSet[RegisteredRoute{Method: r.Method, Path: r.Path}] {
			missing = append(missing, r.Method+" "+r.Path)
		}
	}

	if len(extra) == 0 && len(missing) == 0 {
		return nil
	}

	sort.Strings(extra)
	sort.Strings(missing)
	var b strings.Builder
	fmt.Fprintf(&b, "service %q routes diverge from the services/access Registry", service)
	if len(extra) > 0 {
		fmt.Fprintf(&b, "\n  registered but not in the Registry (add an entry with an Access level, or remove the route): %s", strings.Join(extra, ", "))
	}
	if len(missing) > 0 {
		fmt.Fprintf(&b, "\n  in the Registry but not registered (bind a handler by ID): %s", strings.Join(missing, ", "))
	}
	return fmt.Errorf("%s", b.String())
}
