package access

import "testing"

// TestResolve_WorkedExamples ports the acceptance-criteria worked examples from
// the former gateway resolver suite to the Registry-backed pattern resolver.
// Each is a real route, so the outcome must be preserved exactly.
func TestResolve_WorkedExamples(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   Access
	}{
		{"login is public", "POST", "/api/auth/login", Public},
		{"register is public", "POST", "/api/auth/register", Public},
		{"refresh is public", "POST", "/api/auth/refresh", Public},
		{"me get is authenticated", "GET", "/api/auth/me", Authenticated},
		{"me update is authenticated", "PUT", "/api/auth/me", Authenticated},
		{"me password is authenticated", "POST", "/api/auth/me/password", Authenticated},
		{"logout is authenticated", "POST", "/api/auth/logout", Authenticated},
		{"restore is authenticated", "POST", "/api/auth/restore", Authenticated},
		{"onboarding-complete is personal", "POST", "/api/auth/onboarding-complete", Personal},
		{"assume is admin", "POST", "/api/auth/assume", Admin},
		{"admin users is admin", "GET", "/api/admin/users", Admin},
		{"finance periods is personal", "GET", "/api/finance/periods", Personal},
		{"finance period update is personal (param)", "PUT", "/api/finance/periods/abc-123", Personal},
		{"finance tag delete is personal (param)", "DELETE", "/api/finance/tags/tag-9", Personal},
		{"exports create is personal", "POST", "/api/datarights/exports", Personal},
		{"export by id is personal (param)", "GET", "/api/datarights/exports/export-456", Personal},
		{"deletions create is admin", "POST", "/api/datarights/deletions", Admin},
		{"deletion by id is admin (param)", "GET", "/api/datarights/deletions/abc-123", Admin},
		{"expense by id is personal (param)", "GET", "/api/expenses/e-1", Personal},
		{"expense history is personal (param)", "GET", "/api/expenses/e-1/history", Personal},
		{"expense prorata group is personal (param)", "GET", "/api/expenses/prorata/g-1", Personal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.method, tc.path); got != tc.want {
				t.Errorf("Resolve(%q, %q) = %s, want %s", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

// TestResolve_FallsBackToDeny covers every way a request misses the Registry:
// wrong method, sibling substrings that must not borrow a neighbor's level,
// bare groups with no exact route, and wholly unknown paths. Each now resolves
// to the deny-by-default fail-safe (403 at the gateway), not Authenticated.
func TestResolve_FallsBackToDeny(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"wrong method on a public exact", "GET", "/api/auth/login"},
		{"wrong method on an admin exact", "GET", "/api/auth/assume"},
		{"finance sibling substring", "GET", "/api/finance-summary"},
		{"expenses sibling substring", "GET", "/api/expenses-report"},
		{"admin sibling substring", "GET", "/api/admin-tools"},
		{"exports sibling substring", "POST", "/api/datarights/exports-admin"},
		{"deletions sibling substring", "DELETE", "/api/datarights/deletions-log"},
		{"bare finance group", "GET", "/api/finance"},
		{"bare admin group", "GET", "/api/admin"},
		{"bare datarights group", "GET", "/api/datarights"},
		{"unknown api path", "GET", "/api/unknown"},
		{"extra trailing segment", "GET", "/api/finance/tags/abc/extra"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.method, tc.path); got != Deny {
				t.Errorf("Resolve(%q, %q) = %s, want Deny", tc.method, tc.path, got)
			}
		})
	}
}

// TestResolve_GinPriorityStaticBeatsParam pins the one real same-method overlap:
// GET /api/expenses/suggestions (static) vs GET /api/expenses/:id (param). Both
// patterns match the concrete "suggestions" path, and the resolver must pick
// the static route gin dispatches to.
func TestResolve_GinPriorityStaticBeatsParam(t *testing.T) {
	const static = "/api/expenses/suggestions"
	const param = "/api/expenses/:id"

	if !matchPattern(static, "/api/expenses/suggestions") {
		t.Fatal("static pattern should match its own concrete path")
	}
	if !matchPattern(param, "/api/expenses/suggestions") {
		t.Fatal("param pattern should also match /api/expenses/suggestions (the overlap under test)")
	}
	if !moreSpecific(static, param) {
		t.Errorf("moreSpecific(%q, %q) = false, want true (static must outrank param)", static, param)
	}
	if moreSpecific(param, static) {
		t.Errorf("moreSpecific(%q, %q) = true, want false (param must not outrank static)", param, static)
	}
}

// TestMatchPattern covers the segment matcher's contract independently of the
// Registry: static equality, single-segment params, catch-all remainder, and
// segment-count mismatches.
func TestMatchPattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"static equal", "/api/finance/tags", "/api/finance/tags", true},
		{"static unequal", "/api/finance/tags", "/api/finance/defaults", false},
		{"static substring is not a match", "/api/datarights/exports", "/api/datarights/exports-admin", false},
		{"param matches one segment", "/api/finance/tags/:id", "/api/finance/tags/abc", true},
		{"param rejects empty segment", "/api/finance/tags/:id", "/api/finance/tags/", false},
		{"param rejects extra segment", "/api/finance/tags/:id", "/api/finance/tags/abc/def", false},
		{"param rejects missing segment", "/api/finance/tags/:id", "/api/finance/tags", false},
		{"nested param", "/api/expenses/:id/history", "/api/expenses/e-1/history", true},
		{"nested param wrong tail", "/api/expenses/:id/history", "/api/expenses/e-1/correct", false},
		{"catch-all absorbs remainder", "/api/x/*rest", "/api/x/a/b/c", true},
		{"catch-all requires boundary", "/api/x/*rest", "/api/x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchPattern(tc.pattern, tc.path); got != tc.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}

// TestMoreSpecific proves the gin-priority ordering used to break overlaps:
// static > param > catch-all, decided at the first differing segment.
func TestMoreSpecific(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"static beats param", "/api/expenses/suggestions", "/api/expenses/:id", true},
		{"param beats catch-all", "/api/x/:id", "/api/x/*rest", true},
		{"static beats catch-all", "/api/x/y", "/api/x/*rest", true},
		{"param does not beat static", "/api/expenses/:id", "/api/expenses/suggestions", false},
		{"first differing segment decides", "/api/:a/static", "/api/static/:b", false},
		{"longer pattern wins on tie", "/api/x/:id/history", "/api/x/:id", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := moreSpecific(tc.a, tc.b); got != tc.want {
				t.Errorf("moreSpecific(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
