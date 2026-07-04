package access

import "strings"

// resolve returns the access level for a method+path.
//
// Precedence:
//  1. Exact match: a Match==Exact rule whose Method matches (or is "") and
//     whose Path equals path.
//  2. Longest prefix: among Match==Prefix rules whose Method matches (or is "")
//     and where path is within the rule Path's segment (see hasPathPrefix),
//     the one with the longest Path.
//  3. Default: Policy.Default (Authenticated, the fail-safe).
//
// resolve is a pure function with no gin or net/http dependency, so the whole
// access model can be exhaustively table-tested without a server.
func (p Policy) resolve(method, path string) Access {
	for _, rule := range p.rules {
		if rule.Match == Exact && methodMatches(rule.Method, method) && rule.Path == path {
			return rule.Access
		}
	}

	bestLen := -1
	level := p.Default
	for _, rule := range p.rules {
		if rule.Match != Prefix || !methodMatches(rule.Method, method) {
			continue
		}
		if hasPathPrefix(path, rule.Path) && len(rule.Path) > bestLen {
			bestLen = len(rule.Path)
			level = rule.Access
		}
	}
	return level
}

// methodMatches reports whether a rule's method constraint is satisfied by the
// request method. An empty rule method means "any method".
func methodMatches(ruleMethod, requestMethod string) bool {
	return ruleMethod == "" || ruleMethod == requestMethod
}

// hasPathPrefix reports whether path lies within prefix's path segment: prefix
// must match on a segment boundary, not merely as a leading substring. So
// "/api/finance" matches "/api/finance" and "/api/finance/periods" but NOT
// "/api/finance-summary". Without this boundary a future sibling such as
// "/api/datarights/exports-admin" would match the Personal prefix
// "/api/datarights/exports" and be under-restricted; since this resolver is the
// single authz gate, that would be a direct authz bug.
//
// Policy prefixes are segment paths with no trailing slash (see policy.go), so
// the boundary is the character immediately after the prefix: either the end
// of the path or a '/'.
func hasPathPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}
