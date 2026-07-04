// Package access owns the GoFin gateway access model as pure, gin-free code.
//
// It defines the access levels applied to every gateway route, the rule and
// policy types that encode who may reach what, and a pure resolver that maps a
// method+path to an access level. Keeping this package free of gin and
// net/http lets the entire access model be exhaustively table-tested without a
// running server.
package access

// Access is the classification applied to every gateway route. It determines
// whether a request needs a token and, if so, which role may proceed.
type Access int

const (
	// Public routes are reachable with no token.
	Public Access = iota
	// Authenticated routes require any valid token, regardless of role.
	Authenticated
	// Personal routes require a valid token acting as a regular user (role == "user").
	Personal
	// Admin routes require a valid token acting as an operator (role == "admin").
	Admin
)

// String returns the level name, used for readable logs and test diagnostics.
func (a Access) String() string {
	switch a {
	case Public:
		return "Public"
	case Authenticated:
		return "Authenticated"
	case Personal:
		return "Personal"
	case Admin:
		return "Admin"
	default:
		return "Unknown"
	}
}

// MatchType selects exact vs prefix matching for a Rule.
type MatchType int

const (
	// Exact matches when the method and path are identical to the rule.
	Exact MatchType = iota
	// Prefix matches when the request path starts with the rule path.
	Prefix
)

// Rule maps a method+path pattern to an access level.
type Rule struct {
	// Method is the HTTP method to match. "" means any method (only meaningful
	// for Prefix rules and method-agnostic exacts).
	Method string
	// Path is the exact path (Exact) or the path prefix (Prefix) to match.
	Path string
	// Match selects exact vs prefix matching.
	Match MatchType
	// Access is the level granted when this rule matches.
	Access Access
}

// Policy is the ordered list of rules plus the fallback for unmatched paths.
type Policy struct {
	rules   []Rule
	Default Access // Authenticated (fail-safe)
}
