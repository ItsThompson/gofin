package access

import "net/http"

// Route is one concrete gateway-facing endpoint. Every route carries a
// mandatory Access, so classifying a route is inseparable from declaring it:
// a service cannot register a route (bind-by-ID from RoutesFor) without the
// Registry entry that also states who may reach it.
type Route struct {
	// ID is a stable, unique key (e.g. "auth.login", "finance.periods.update").
	// Services bind their handlers to routes by this ID.
	ID string
	// Service is the downstream service that registers the route
	// ("auth" | "finance" | "expense" | "datarights"). /api/admin/users is
	// registered by auth, so its Service is "auth" even though its Access is Admin.
	Service string
	// Method is the HTTP method (http.MethodGet, http.MethodPost, ...).
	Method string
	// Path is the full gin path pattern including params, e.g.
	// "/api/finance/tags/:id" or "/api/expenses/:id/history".
	Path string
	// Access is the classification enforced by the gateway middleware.
	Access Level
}

// registry is the single, exhaustive list of every gateway-facing downstream
// route. It is the sole source of truth, set once at init and unexported so no
// caller can reassign it: downstream services read it via RoutesFor and the
// gateway classifies each request via Resolve (see resolver.go).
//
// Gateway-native endpoints (/health, /metrics) are intentionally absent: no
// downstream service serves them, so the gateway classifies them itself.
//
// Path strings must match gin's registered patterns byte-for-byte (verified by
// each service's registration coverage test against engine.Routes()).
var registry = []Route{
	// --- auth service ---
	{ID: "auth.register", Service: "auth", Method: http.MethodPost, Path: "/api/auth/register", Access: Public},
	{ID: "auth.login", Service: "auth", Method: http.MethodPost, Path: "/api/auth/login", Access: Public},
	{ID: "auth.refresh", Service: "auth", Method: http.MethodPost, Path: "/api/auth/refresh", Access: Public},
	{ID: "auth.logout", Service: "auth", Method: http.MethodPost, Path: "/api/auth/logout", Access: Authenticated},
	{ID: "auth.me.get", Service: "auth", Method: http.MethodGet, Path: "/api/auth/me", Access: Authenticated},
	{ID: "auth.me.update", Service: "auth", Method: http.MethodPut, Path: "/api/auth/me", Access: Authenticated},
	{ID: "auth.me.password", Service: "auth", Method: http.MethodPost, Path: "/api/auth/me/password", Access: Authenticated},
	{ID: "auth.onboarding_complete", Service: "auth", Method: http.MethodPost, Path: "/api/auth/onboarding-complete", Access: Personal},
	{ID: "auth.assume", Service: "auth", Method: http.MethodPost, Path: "/api/auth/assume", Access: Admin},
	{ID: "auth.restore", Service: "auth", Method: http.MethodPost, Path: "/api/auth/restore", Access: Authenticated},
	{ID: "admin.users.list", Service: "auth", Method: http.MethodGet, Path: "/api/admin/users", Access: Admin},

	// --- finance service ---
	{ID: "finance.onboarding", Service: "finance", Method: http.MethodPost, Path: "/api/finance/onboarding", Access: Personal},
	{ID: "finance.defaults.get", Service: "finance", Method: http.MethodGet, Path: "/api/finance/defaults", Access: Personal},
	{ID: "finance.currencies.list", Service: "finance", Method: http.MethodGet, Path: "/api/finance/currencies", Access: Authenticated},
	{ID: "finance.defaults.update", Service: "finance", Method: http.MethodPut, Path: "/api/finance/defaults", Access: Personal},
	{ID: "finance.periods.current", Service: "finance", Method: http.MethodGet, Path: "/api/finance/periods/current", Access: Personal},
	{ID: "finance.periods.list", Service: "finance", Method: http.MethodGet, Path: "/api/finance/periods", Access: Personal},
	{ID: "finance.periods.create", Service: "finance", Method: http.MethodPost, Path: "/api/finance/periods", Access: Personal},
	{ID: "finance.periods.update", Service: "finance", Method: http.MethodPut, Path: "/api/finance/periods/:id", Access: Personal},
	{ID: "finance.tags.list", Service: "finance", Method: http.MethodGet, Path: "/api/finance/tags", Access: Personal},
	{ID: "finance.tags.create", Service: "finance", Method: http.MethodPost, Path: "/api/finance/tags", Access: Personal},
	{ID: "finance.tags.update", Service: "finance", Method: http.MethodPut, Path: "/api/finance/tags/:id", Access: Personal},
	{ID: "finance.tags.delete", Service: "finance", Method: http.MethodDelete, Path: "/api/finance/tags/:id", Access: Personal},
	{ID: "finance.summary", Service: "finance", Method: http.MethodGet, Path: "/api/finance/summary", Access: Personal},
	{ID: "finance.spending.by_tag", Service: "finance", Method: http.MethodGet, Path: "/api/finance/spending/by-tag", Access: Personal},
	{ID: "finance.spending.cumulative", Service: "finance", Method: http.MethodGet, Path: "/api/finance/spending/cumulative", Access: Personal},
	{ID: "finance.spending.comparison", Service: "finance", Method: http.MethodGet, Path: "/api/finance/spending/comparison", Access: Personal},
	{ID: "finance.spending.trends", Service: "finance", Method: http.MethodGet, Path: "/api/finance/spending/trends", Access: Personal},
	{ID: "finance.prorata.create", Service: "finance", Method: http.MethodPost, Path: "/api/finance/prorata", Access: Personal},
	{ID: "finance.prorata.upcoming", Service: "finance", Method: http.MethodGet, Path: "/api/finance/prorata/upcoming", Access: Personal},
	{ID: "finance.health_score", Service: "finance", Method: http.MethodGet, Path: "/api/finance/health-score", Access: Personal},
	{ID: "finance.health_score.trend", Service: "finance", Method: http.MethodGet, Path: "/api/finance/health-score/trend", Access: Personal},

	// --- expense service ---
	{ID: "expense.create", Service: "expense", Method: http.MethodPost, Path: "/api/expenses", Access: Personal},
	{ID: "expense.list", Service: "expense", Method: http.MethodGet, Path: "/api/expenses", Access: Personal},
	{ID: "expense.suggestions", Service: "expense", Method: http.MethodGet, Path: "/api/expenses/suggestions", Access: Personal},
	{ID: "expense.prorata.group", Service: "expense", Method: http.MethodGet, Path: "/api/expenses/prorata/:groupId", Access: Personal},
	{ID: "expense.get", Service: "expense", Method: http.MethodGet, Path: "/api/expenses/:id", Access: Personal},
	{ID: "expense.correct", Service: "expense", Method: http.MethodPost, Path: "/api/expenses/:id/correct", Access: Personal},
	{ID: "expense.delete", Service: "expense", Method: http.MethodDelete, Path: "/api/expenses/:id", Access: Personal},
	{ID: "expense.history", Service: "expense", Method: http.MethodGet, Path: "/api/expenses/:id/history", Access: Personal},

	// --- datarights service ---
	{ID: "datarights.exports.create", Service: "datarights", Method: http.MethodPost, Path: "/api/datarights/exports", Access: Personal},
	{ID: "datarights.exports.list", Service: "datarights", Method: http.MethodGet, Path: "/api/datarights/exports", Access: Personal},
	{ID: "datarights.exports.get", Service: "datarights", Method: http.MethodGet, Path: "/api/datarights/exports/:id", Access: Personal},
	{ID: "datarights.deletions.create", Service: "datarights", Method: http.MethodPost, Path: "/api/datarights/deletions", Access: Admin},
	{ID: "datarights.deletions.get", Service: "datarights", Method: http.MethodGet, Path: "/api/datarights/deletions/:id", Access: Admin},
}

// RoutesFor returns every Registry route registered by the named service, in
// Registry order. Downstream services iterate this to bind handlers by ID, so
// a route missing from the Registry can never be served.
func RoutesFor(service string) []Route {
	var routes []Route
	for _, r := range registry {
		if r.Service == service {
			routes = append(routes, r)
		}
	}
	return routes
}

// RouteID returns the ID of the route registered for method and pattern, or ""
// when the Registry holds none. pattern is gin's registered path pattern
// (c.FullPath()), which each service's registration coverage test already pins to
// Route.Path byte-for-byte, so this is an exact match rather than a second copy of
// Resolve's precedence rules.
//
// Error reporting tags each event with the failing operation, and it needs a
// bounded name per route. The IDs are exactly that and they already exist, so a
// reporter derives the name here instead of every handler restating it.
func RouteID(method, pattern string) string {
	for _, r := range registry {
		if r.Method == method && r.Path == pattern {
			return r.ID
		}
	}
	return ""
}
