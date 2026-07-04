package access

import "testing"

// Route coverage guardrail.
//
// This file is a HAND-MAINTAINED enumeration of every known gateway route
// class. For each one it asserts two things: the route resolves to its intended
// access level, and that level comes from an explicit rule rather than the
// fail-safe Default (Authenticated). It turns "someone added a route but forgot
// to classify it" into a CI failure instead of an accidental exposure.
//
// It is intentionally separate from resolver_test.go: that suite proves
// precedence correctness (exact > longest prefix > default); this suite proves
// surface coverage (every real route is classified by a rule).
//
// Residual risk: because the enumeration is maintained by hand, it only guards
// routes someone remembered to list here. The whole-service prefixes
// (/api/finance, /api/expenses, /api/datarights/exports, /api/datarights/deletions,
// /api/admin) already cover every subpath beneath them, so new endpoints in
// those services are classified automatically. The real exposure is a NEW
// mixed-group route added under /api/auth or /api/datarights (the only groups
// that mix access levels): such a route must be added to BOTH the policy table
// (policy.go) and this enumeration. Forgetting the policy entry makes this test
// fail (the route resolves to the Default rather than its intended level);
// forgetting to also list it here is the residual gap this guardrail cannot
// close.

// sentinelDefault is an Access value that no rule in DefaultPolicy() ever
// returns. Resolving a route under a policy whose Default is this sentinel lets
// the tests below tell "classified by a rule" apart from "fell through to the
// Default": a rule-classified route ignores Default and resolves identically
// under any Default, while an unclassified route returns whatever Default is
// configured.
const sentinelDefault Access = -1

// classifiedByRule reports whether method+path is matched by an explicit rule
// in DefaultPolicy(), as opposed to falling through to the policy Default.
//
// It resolves the route twice against policies that are identical except for
// their Default value. If a rule matches, both resolutions return that rule's
// level and agree; if no rule matches, each returns its own Default and they
// disagree. This distinguishes an explicit classification from the fail-safe
// fallback without changing the access package, which matters for routes whose
// intended level (Authenticated) happens to equal the real Default.
func classifiedByRule(method, path string) bool {
	withRealDefault := DefaultPolicy()
	withSentinelDefault := DefaultPolicy()
	withSentinelDefault.Default = sentinelDefault
	return withRealDefault.resolve(method, path) == withSentinelDefault.resolve(method, path)
}

// TestCoverage_KnownRoutesResolveToExplicitLevel enumerates every known gateway
// route class and asserts each resolves to its intended, rule-classified access
// level. See the file header for the hand-maintained-enumeration caveat and its
// residual risk.
//
// For method-agnostic prefix classes (/api/admin, /api/datarights/*,
// /api/finance, /api/expenses) the method column carries a representative real
// method; classification is method-independent for those (proven in
// resolver_test.go), so any method resolves the same.
func TestCoverage_KnownRoutesResolveToExplicitLevel(t *testing.T) {
	policy := DefaultPolicy()

	cases := []struct {
		name   string
		method string
		path   string
		want   Access
	}{
		// Public: reachable with no token.
		{"register", "POST", "/api/auth/register", Public},
		{"login", "POST", "/api/auth/login", Public},
		{"refresh", "POST", "/api/auth/refresh", Public},
		{"health", "GET", "/health", Public},
		{"metrics", "GET", "/metrics", Public},

		// Authenticated: any valid token, no role check. These live in the
		// mixed /api/auth group, so their intended level equals the Default;
		// the classifiedByRule assertion below is what proves each is
		// rule-classified rather than a silent fall-through.
		{"me (GET)", "GET", "/api/auth/me", Authenticated},
		{"me (PUT)", "PUT", "/api/auth/me", Authenticated},
		{"me password", "POST", "/api/auth/me/password", Authenticated},
		{"logout", "POST", "/api/auth/logout", Authenticated},
		{"restore", "POST", "/api/auth/restore", Authenticated},

		// Admin: operator-only.
		{"assume", "POST", "/api/auth/assume", Admin},
		{"admin group", "GET", "/api/admin", Admin},
		{"datarights deletions", "DELETE", "/api/datarights/deletions", Admin},

		// Personal: valid token acting as a regular user.
		{"onboarding-complete", "POST", "/api/auth/onboarding-complete", Personal},
		{"finance group", "GET", "/api/finance", Personal},
		{"expenses group", "GET", "/api/expenses", Personal},
		{"datarights exports", "POST", "/api/datarights/exports", Personal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := policy.resolve(tc.method, tc.path); got != tc.want {
				t.Errorf("resolve(%q, %q) = %s, want %s", tc.method, tc.path, got, tc.want)
			}
			if !classifiedByRule(tc.method, tc.path) {
				t.Errorf("route %s %q is not classified by an explicit rule; it falls through to the policy Default (%s). Add a matching rule in policy.go.", tc.method, tc.path, policy.Default)
			}
		})
	}
}

// TestCoverage_UnclassifiedRouteFallsThroughToDefault demonstrates the
// guardrail's failure mode, satisfying the acceptance criterion that adding a
// personal- or admin-intended route with no matching policy entry causes the
// coverage test to fail.
//
// A route with no rule resolves to the fail-safe Default (Authenticated) and is
// not classified by a rule. If such a route were added to the enumeration above
// with its intended non-default level (Personal or Admin), the want assertion
// would fail (got Authenticated), surfacing the missing classification in CI.
func TestCoverage_UnclassifiedRouteFallsThroughToDefault(t *testing.T) {
	policy := DefaultPolicy()

	// A hypothetical new downstream route with no matching policy entry.
	const method, path = "GET", "/api/newservice/records"

	if classifiedByRule(method, path) {
		t.Fatalf("expected %q to be unclassified, but a rule matched it", path)
	}
	if got := policy.resolve(method, path); got != policy.Default {
		t.Errorf("unclassified route should fall to Default %s, got %s", policy.Default, got)
	}
	// Because the Default is Authenticated, an intended-Personal or -Admin route
	// left unclassified would fail the coverage table's want assertion.
	if got := policy.resolve(method, path); got == Personal || got == Admin {
		t.Errorf("unclassified route should not resolve to a role-gated level, got %s", got)
	}
}
