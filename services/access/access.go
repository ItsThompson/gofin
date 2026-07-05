// Package access is the single source of truth for GoFin's gateway-facing
// route surface and its access model. It is deliberately free of gin and
// net/http server code so both the gateway (which derives its authz policy
// from the Registry) and every downstream service (which registers its routes
// from the Registry) can import it without a build dependency on each other.
//
// The package owns three things:
//   - the Access classification applied to every route (access.go),
//   - the Registry of concrete routes and their access levels (registry.go),
//   - a gin-priority path resolver used by the gateway middleware (resolver.go).
package access

// Access is the classification applied to every gateway-facing route. It
// determines whether a request needs a token and, if so, which role may
// proceed.
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
