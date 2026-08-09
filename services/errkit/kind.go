package errkit

// Kind is the low-cardinality classification of a failure. Every value becomes
// the Sentry tag error_kind and part of the grouping key, so this set is closed:
// adding a value widens the query vocabulary of both gofin Sentry projects.
type Kind string

const (
	KindValidation Kind = "validation"
	KindNotFound   Kind = "not_found"
	KindConflict   Kind = "conflict"
	KindPermission Kind = "permission"
	KindUpstream   Kind = "upstream"
	KindTimeout    Kind = "timeout"
	KindDatabase   Kind = "database"
	KindInternal   Kind = "internal"
)

// resolve returns k, or KindInternal when k is empty. It runs before the group
// key is derived, so a report with no Kind groups under "internal" rather than
// under a key with an empty half.
func (k Kind) resolve() Kind {
	if k == "" {
		return KindInternal
	}
	return k
}
