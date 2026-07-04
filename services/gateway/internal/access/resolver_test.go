package access

import "testing"

// TestResolve_DefaultPolicy covers the section-4 worked examples plus
// exhaustive cases across every rule in the canonical policy table: Public
// exacts, method-agnostic prefixes, exact-over-default precedence, longest
// prefix ranking, and the Authenticated fail-safe default.
func TestResolve_DefaultPolicy(t *testing.T) {
	policy := DefaultPolicy()

	cases := []struct {
		name   string
		method string
		path   string
		want   Access
	}{
		// --- Section-4 worked examples (acceptance criteria) ---
		{"login is public", "POST", "/api/auth/login", Public},
		{"me is authenticated (prefix)", "GET", "/api/auth/me", Authenticated},
		{"me password is authenticated (prefix)", "POST", "/api/auth/me/password", Authenticated},
		{"onboarding-complete is personal (exact)", "POST", "/api/auth/onboarding-complete", Personal},
		{"assume is admin (exact)", "POST", "/api/auth/assume", Admin},
		{"restore is authenticated (exact)", "POST", "/api/auth/restore", Authenticated},
		{"finance periods is personal (prefix)", "GET", "/api/finance/periods", Personal},
		{"exports is personal (prefix)", "POST", "/api/datarights/exports", Personal},
		{"deletions by id is admin (longest prefix)", "DELETE", "/api/datarights/deletions/abc-123", Admin},
		{"admin users is admin (prefix)", "GET", "/api/admin/users", Admin},
		{"bare auth group falls to default", "GET", "/api/auth", Authenticated},

		// --- Remaining Public exacts ---
		{"register is public", "POST", "/api/auth/register", Public},
		{"refresh is public", "POST", "/api/auth/refresh", Public},
		{"health is public", "GET", "/health", Public},
		{"metrics is public", "GET", "/metrics", Public},

		// --- Remaining Authenticated exacts ---
		{"logout is authenticated (exact)", "POST", "/api/auth/logout", Authenticated},

		// --- Personal whole-service prefixes ---
		{"finance bare group is personal", "GET", "/api/finance", Personal},
		{"expenses is personal (prefix)", "POST", "/api/expenses", Personal},
		{"expenses subpath is personal", "GET", "/api/expenses/123", Personal},
		{"exports subpath is personal (longest prefix)", "GET", "/api/datarights/exports/export-456", Personal},

		// --- Admin prefixes ---
		{"admin bare group is admin", "GET", "/api/admin", Admin},
		{"deletions bare is admin (prefix)", "POST", "/api/datarights/deletions", Admin},

		// --- Method sensitivity: an exact rule's method must match ---
		{"wrong method on public exact falls to default", "GET", "/api/auth/login", Authenticated},
		{"wrong method on admin exact falls to default", "GET", "/api/auth/assume", Authenticated},

		// --- Default fallback for unclassified paths ---
		{"bare datarights falls to default", "GET", "/api/datarights", Authenticated},
		{"unknown datarights subpath falls to default", "GET", "/api/datarights/unknown", Authenticated},
		{"unknown api path falls to default", "GET", "/api/unknown", Authenticated},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.resolve(tc.method, tc.path)
			if got != tc.want {
				t.Errorf("resolve(%q, %q) = %s, want %s", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

// TestResolve_ExactBeatsPrefix proves the exact pass runs before the prefix
// pass: even when a broader prefix rule would match the same path, the exact
// rule wins regardless of table order.
func TestResolve_ExactBeatsPrefix(t *testing.T) {
	policy := Policy{
		Default: Authenticated,
		rules: []Rule{
			{Method: "", Path: "/api/auth", Match: Prefix, Access: Personal},
			{Method: "POST", Path: "/api/auth/assume", Match: Exact, Access: Admin},
		},
	}

	if got := policy.resolve("POST", "/api/auth/assume"); got != Admin {
		t.Errorf("exact rule should win over prefix: got %s, want %s", got, Admin)
	}
	// A sibling path with no exact rule falls to the prefix rule.
	if got := policy.resolve("GET", "/api/auth/other"); got != Personal {
		t.Errorf("prefix rule should apply when no exact matches: got %s, want %s", got, Personal)
	}
}

// TestResolve_LongestPrefixWins proves the resolver picks the longest matching
// prefix, independent of the order rules appear in the table.
func TestResolve_LongestPrefixWins(t *testing.T) {
	// Shorter prefix listed AFTER the longer one to ensure ordering is not
	// what selects the winner.
	policy := Policy{
		Default: Authenticated,
		rules: []Rule{
			{Method: "", Path: "/api/datarights/deletions", Match: Prefix, Access: Admin},
			{Method: "", Path: "/api/datarights", Match: Prefix, Access: Personal},
		},
	}

	if got := policy.resolve("DELETE", "/api/datarights/deletions/1"); got != Admin {
		t.Errorf("longest prefix should win: got %s, want %s", got, Admin)
	}
	if got := policy.resolve("GET", "/api/datarights/other"); got != Personal {
		t.Errorf("shorter prefix should apply when longer does not match: got %s, want %s", got, Personal)
	}
}

// TestResolve_DefaultFallback confirms an empty policy (and any unmatched path)
// returns the configured Default.
func TestResolve_DefaultFallback(t *testing.T) {
	policy := Policy{Default: Authenticated}
	if got := policy.resolve("GET", "/anything"); got != Authenticated {
		t.Errorf("empty policy should return Default: got %s, want %s", got, Authenticated)
	}

	custom := Policy{Default: Public}
	if got := custom.resolve("GET", "/anything"); got != Public {
		t.Errorf("Default should be honored: got %s, want %s", got, Public)
	}
}

// TestResolve_MethodAgnosticPrefix confirms a prefix rule with an empty method
// matches every HTTP method, while a method-scoped exact only matches its
// method.
func TestResolve_MethodAgnosticPrefix(t *testing.T) {
	policy := DefaultPolicy()

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if got := policy.resolve(method, "/api/admin/users"); got != Admin {
			t.Errorf("method-agnostic prefix should match %s: got %s, want %s", method, got, Admin)
		}
	}
}
