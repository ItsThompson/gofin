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

// ProxyPrefixes is the ordered inventory of downstream prefixes the gateway
// proxies and the service each targets. It makes "what prefixes and services
// exist" part of the single source of truth: the gateway derives its proxy
// wiring from this list (see services/gateway/internal/router), and a
// cross-check test pins it against the Registry so every classified route sits
// under a proxied prefix and every proxied prefix has at least one classified
// route.
//
// Two prefixes map to the auth service: /api/auth (its own routes) and
// /api/admin (operator routes such as GET /api/admin/users, still served by
// auth even though their Access is Admin).
var ProxyPrefixes = []ProxyPrefix{
	{Prefix: "/api/auth", Service: "auth"},
	{Prefix: "/api/admin", Service: "auth"},
	{Prefix: "/api/expenses", Service: "expense"},
	{Prefix: "/api/finance", Service: "finance"},
	{Prefix: "/api/datarights", Service: "datarights"},
}
