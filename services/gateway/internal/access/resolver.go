package access

import "strings"

// resolve returns the access level for a method+path.
//
// Precedence:
//  1. Exact match: a Match==Exact rule whose Method matches (or is "") and
//     whose Path equals path.
//  2. Longest prefix: among Match==Prefix rules whose Method matches (or is "")
//     and where path starts with the rule Path, the one with the longest Path.
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
	access := p.Default
	for _, rule := range p.rules {
		if rule.Match != Prefix || !methodMatches(rule.Method, method) {
			continue
		}
		if strings.HasPrefix(path, rule.Path) && len(rule.Path) > bestLen {
			bestLen = len(rule.Path)
			access = rule.Access
		}
	}
	return access
}

// methodMatches reports whether a rule's method constraint is satisfied by the
// request method. An empty rule method means "any method".
func methodMatches(ruleMethod, requestMethod string) bool {
	return ruleMethod == "" || ruleMethod == requestMethod
}
