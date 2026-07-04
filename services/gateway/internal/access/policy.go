package access

// HTTP method constants used by the policy table. These are plain strings
// matching the values gin reports for c.Request.Method (identical to the
// net/http.Method* constants), kept local so the access package stays free of
// any net/http dependency and remains purely table-testable.
const (
	methodGet    = "GET"
	methodPost   = "POST"
	methodDelete = "DELETE"
)

// DefaultPolicy returns the canonical GoFin access policy table. It is the
// single source of truth for who can reach what. Rule order is for
// readability only: the resolver is order-independent within a match class
// (see resolver.go), grouping rules here by access level to mirror the spec.
func DefaultPolicy() Policy {
	return Policy{
		Default: Authenticated, // fail-safe: unmatched routes require any valid token
		rules: []Rule{
			// Public: reachable with no token.
			{Method: methodPost, Path: "/api/auth/register", Match: Exact, Access: Public},
			{Method: methodPost, Path: "/api/auth/login", Match: Exact, Access: Public},
			{Method: methodPost, Path: "/api/auth/refresh", Match: Exact, Access: Public},
			{Method: methodGet, Path: "/health", Match: Exact, Access: Public},
			{Method: methodGet, Path: "/metrics", Match: Exact, Access: Public},

			// Admin: operator-only.
			{Method: "", Path: "/api/admin", Match: Prefix, Access: Admin},
			{Method: methodPost, Path: "/api/auth/assume", Match: Exact, Access: Admin},
			{Method: "", Path: "/api/datarights/deletions", Match: Prefix, Access: Admin},

			// Personal: valid token acting as a regular user (role == "user").
			{Method: methodPost, Path: "/api/auth/onboarding-complete", Match: Exact, Access: Personal},
			{Method: "", Path: "/api/finance", Match: Prefix, Access: Personal},
			{Method: "", Path: "/api/expenses", Match: Prefix, Access: Personal},
			{Method: "", Path: "/api/datarights/exports", Match: Prefix, Access: Personal},

			// Authenticated: any valid token, no role check.
			{Method: "", Path: "/api/auth/me", Match: Prefix, Access: Authenticated},
			{Method: methodPost, Path: "/api/auth/logout", Match: Exact, Access: Authenticated},
			{Method: methodPost, Path: "/api/auth/restore", Match: Exact, Access: Authenticated},
		},
	}
}
