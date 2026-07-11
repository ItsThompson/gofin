package access

import (
	"strings"
	"testing"
)

// underPrefix reports whether a concrete route path sits under a proxy prefix
// on a segment boundary: an exact match, or the prefix followed by "/". This
// mirrors how the gateway groups proxy a prefix and its subtree, and rejects
// leading-substring siblings like "/api/expenses-report" under "/api/expenses".
func underPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// coveringPrefix returns the single ProxyPrefix a path sits under (prefixes are
// disjoint on segment boundaries, so at most one matches).
func coveringPrefix(path string) (ProxyPrefix, bool) {
	for _, p := range proxyPrefixes {
		if underPrefix(path, p.Prefix) {
			return p, true
		}
	}
	return ProxyPrefix{}, false
}

// TestProxyPrefixes_EveryRegistryRouteIsReachable is direction one of the
// single-source cross-check: every classified route must sit under a proxied
// prefix (no classified-but-unreachable route), and that prefix must proxy the
// same service that serves the route (else the gateway would forward it to the
// wrong downstream).
func TestProxyPrefixes_EveryRegistryRouteIsReachable(t *testing.T) {
	for _, r := range registry {
		prefix, ok := coveringPrefix(r.Path)
		if !ok {
			t.Errorf("route %s (%s %s) is classified but sits under no ProxyPrefix; the gateway would never proxy it (add a ProxyPrefix for its prefix)", r.ID, r.Method, r.Path)
			continue
		}
		if prefix.Service != r.Service {
			t.Errorf("route %s (%s) is served by %q but its prefix %q proxies to %q; the gateway would route it to the wrong service", r.ID, r.Path, r.Service, prefix.Prefix, prefix.Service)
		}
	}
}

// TestProxyPrefixes_EveryPrefixHasARoute is direction two of the cross-check:
// every proxied prefix must have at least one classified route under it. A
// prefix with no Registry route is proxied-but-unclassified, so under
// deny-by-default every request to it would 403 (a silently dead prefix).
func TestProxyPrefixes_EveryPrefixHasARoute(t *testing.T) {
	for _, p := range proxyPrefixes {
		found := false
		for _, r := range registry {
			if underPrefix(r.Path, p.Prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ProxyPrefix %q (-> %q) has no Registry route under it; it proxies an unclassified prefix where every request would be denied (add a Registry entry or remove the prefix)", p.Prefix, p.Service)
		}
	}
}
