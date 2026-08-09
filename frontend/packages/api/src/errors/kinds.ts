/**
 * Low-cardinality failure classification. Every value becomes the Sentry tag
 * `error_kind`, so the union must stay closed: an unbounded value would make
 * the tag's distribution page useless.
 *
 * The Go side declares the same set minus `network` and `parse` (a Go transport
 * failure is an `upstream` or `timeout` failure at the call site that owns it)
 * and plus `database`. The two definitions stay in sync by review.
 */
export type ErrorKind =
  | "validation"
  | "not_found"
  | "conflict"
  | "permission"
  | "upstream"
  | "timeout"
  | "network"
  | "parse"
  | "internal";
