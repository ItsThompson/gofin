package access

import "fmt"

// BindRoutes registers every Registry route for a service by looking its
// handler up by ID and handing the method+path+handler to register. It is the
// single fail-fast binding point shared by every downstream service: a Registry
// entry with no handler panics (a programming error caught at startup and in
// the per-service registration test), so a route can never be served without
// being classified, and no service hand-maintains its own route list.
//
// It is generic over the handler type so the access module stays free of any
// web-framework dependency; callers pass gin.HandlerFunc (via an engine.Handle
// adapter) as H.
func BindRoutes[H any](service string, handlers map[string]H, register func(method, path string, handler H)) {
	for _, r := range RoutesFor(service) {
		handler, ok := handlers[r.ID]
		if !ok {
			panic(fmt.Sprintf(
				"no handler bound for route %s (%s %s); add it to the service handler map or the services/access Registry",
				r.ID, r.Method, r.Path,
			))
		}
		register(r.Method, r.Path, handler)
	}
}
