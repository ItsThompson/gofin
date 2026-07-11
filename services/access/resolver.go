package access

import "strings"

// Resolve returns the access level the gateway must enforce for a concrete
// request method+path. It matches the path against every Registry pattern of
// the same method and, when several match, picks the one gin itself would
// dispatch to (see moreSpecific), so the gateway classifies exactly the route
// that will handle the request.
//
// Unmatched requests fall back to Deny, the fail-safe default: a path that no
// Registry entry classifies is denied (403), not allowed. This makes route
// classification self-enforcing across services as well as routes: a new or
// unclassified route (or a whole new proxied prefix) is dead on arrival until
// it is added to the Registry with an access level, extending the existing
// "can't ship an unclassified route" guarantee to new services by
// construction. Because every Registry pattern is a concrete route with exact
// static segments, a sibling path like "/api/datarights/exports-admin" cannot
// borrow "/api/datarights/exports"'s level: static segments must match
// byte-for-byte. This makes the leading-substring bug that segment-boundary
// prefix matching guarded against (commit ca37e4c) impossible by construction.
func Resolve(method, path string) Level {
	var best *Route
	for i := range Registry {
		entry := &Registry[i]
		if entry.Method != method || !matchPattern(entry.Path, path) {
			continue
		}
		if best == nil || moreSpecific(entry.Path, best.Path) {
			best = entry
		}
	}
	if best == nil {
		return Deny
	}
	return best.Access
}

// matchPattern reports whether a concrete path matches a gin route pattern.
// A static segment must equal the path segment exactly; a ":name" param
// matches exactly one non-empty segment; a "*name" catch-all matches the
// remainder of the path. Segment counts must line up unless a catch-all
// absorbs the rest.
func matchPattern(pattern, path string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(path, "/")

	for i, seg := range patternSegments {
		if strings.HasPrefix(seg, "*") {
			// Catch-all absorbs this segment and everything after it. gin
			// requires at least the boundary slash, i.e. one more segment.
			return len(pathSegments) > i
		}
		if i >= len(pathSegments) {
			return false
		}
		if strings.HasPrefix(seg, ":") {
			if pathSegments[i] == "" {
				return false // a param must match a non-empty segment
			}
			continue
		}
		if seg != pathSegments[i] {
			return false
		}
	}
	return len(pathSegments) == len(patternSegments)
}

// moreSpecific reports whether pattern a outranks pattern b under gin's routing
// priority: comparing segment classes left to right, static (2) beats param
// (1) beats catch-all (0), and the first differing segment decides. This
// mirrors how gin picks a handler when patterns overlap, so the gateway
// classifies the same route gin dispatches to. The concrete overlap it settles
// is GET /api/expenses/suggestions (static) vs /api/expenses/:id (param).
func moreSpecific(a, b string) bool {
	aSegments := strings.Split(a, "/")
	bSegments := strings.Split(b, "/")

	shared := min(len(aSegments), len(bSegments))
	for i := range shared {
		aClass := segmentClass(aSegments[i])
		bClass := segmentClass(bSegments[i])
		if aClass != bClass {
			return aClass > bClass
		}
	}
	// Equal classes across the shared prefix: the longer pattern is more
	// specific (it constrains more segments).
	return len(aSegments) > len(bSegments)
}

// segmentClass scores a pattern segment by gin priority: static > param > catch-all.
func segmentClass(seg string) int {
	switch {
	case strings.HasPrefix(seg, "*"):
		return 0
	case strings.HasPrefix(seg, ":"):
		return 1
	default:
		return 2
	}
}
