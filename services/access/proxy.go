package access

// ProxyPrefix ties a gateway URL prefix to the downstream service that serves
// every route under it. It is the routing half of the single source of truth:
// the Registry says how each route is classified, ProxyPrefixes says which
// service the gateway proxies each prefix to.
type ProxyPrefix struct {
	// Prefix is the gin group prefix the gateway proxies (e.g. "/api/finance").
	Prefix string
	// Service is the downstream service that serves the prefix, matching
	// Route.Service in the Registry ("auth" | "finance" | "expense" | "datarights").
	Service string
}

// proxyPrefixes is the ordered inventory of downstream prefixes the gateway
// proxies and the service each targets. It makes "what prefixes and services
// exist" part of the single source of truth: the gateway derives its proxy
// wiring from this list (see services/gateway/internal/router), and a
// cross-check test pins it against the Registry so every classified route sits
// under a proxied prefix and every proxied prefix has at least one classified
// route.
//
// It is set once at init and unexported so no caller can reassign it; the
// gateway reads a copy via Prefixes().
//
// Two prefixes map to the auth service: /api/auth (its own routes) and
// /api/admin (operator routes such as GET /api/admin/users, still served by
// auth even though their Access is Admin).
var proxyPrefixes = []ProxyPrefix{
	{Prefix: "/api/auth", Service: "auth"},
	{Prefix: "/api/admin", Service: "auth"},
	{Prefix: "/api/expenses", Service: "expense"},
	{Prefix: "/api/finance", Service: "finance"},
	{Prefix: "/api/datarights", Service: "datarights"},
}

// Prefixes returns a copy of the proxy-prefix inventory. The copy is why the
// gateway can derive its routing wiring (router.New) from this list without
// being able to mutate the shared, init-time-fixed backing array.
func Prefixes() []ProxyPrefix {
	out := make([]ProxyPrefix, len(proxyPrefixes))
	copy(out, proxyPrefixes)
	return out
}
