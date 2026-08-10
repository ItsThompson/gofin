import * as Sentry from "@sentry/react-router";
import type { SeverityLevel } from "@sentry/react-router";
import type { ErrorKind } from "./kinds";
import type { ReportOptions } from "./types";

const DEFAULT_LEVEL: SeverityLevel = "error";
const DEFAULT_KIND: ErrorKind = "internal";

/** Sentry truncates a tag value at 200 characters; do it here so the value we assert is the value we send. */
const MAX_TAG_VALUE = 200;

/**
 * `slice` counts UTF-16 code units, so a cut at 200 can split a surrogate
 * pair. Unreachable while every tag value is ASCII; the site to fix if not.
 */
function tagValue(value: string | number | boolean): string {
  return String(value).replace(/\n/g, " ").slice(0, MAX_TAG_VALUE);
}

/**
 * The taxonomy owns these keys outright, so `tags` cannot set them even when the
 * matching option is absent. An `expected` tag arriving through `tags` would
 * reach the client's quota filter and drop a real error with nothing at the call
 * site to suggest it, and a caller-set `operation` would carry an unbounded
 * value that disagrees with the fingerprint.
 */
const RESERVED_TAGS = new Set([
  "error_kind",
  "operation",
  "domain",
  "expected",
]);

function resolveTags(
  kind: ErrorKind,
  options: ReportOptions,
): Record<string, string> {
  const raw: Record<string, string | number | boolean> = {};

  for (const [key, value] of Object.entries(options.tags ?? {})) {
    if (!RESERVED_TAGS.has(key)) raw[key] = value;
  }

  raw.error_kind = kind;
  if (options.op) raw.operation = options.op;
  if (options.domain) raw.domain = options.domain;
  if (options.expected) raw.expected = "true";

  return Object.fromEntries(
    Object.entries(raw).map(([key, value]) => [key, tagValue(value)]),
  );
}

/**
 * Keeping "{{ default }}" as the first element means the fingerprint can only
 * refine Sentry's own grouping, never merge two issues it already separated.
 * `groupExact` drops it deliberately, and is ignored without a key so the
 * emitted fingerprint is never empty.
 */
function resolveFingerprint(
  kind: ErrorKind,
  options: ReportOptions,
): string[] {
  if (options.groupExact && options.groupKey) return [options.groupKey];

  const groupKey =
    options.groupKey || (options.op ? `${options.op}/${kind}` : kind);
  return ["{{ default }}", groupKey];
}

/**
 * The single frontend error-reporting path, for both the browser bundle and the
 * SSR process. Returns the Sentry event id.
 *
 * `error` is passed to the SDK exactly as received. Replacing a non-`Error`
 * input with `new Error(String(error))` would mark the exception synthetic and
 * root its stack in this function, and because Sentry excludes the message from
 * grouping whenever a stack is present, every unrelated error would then collapse
 * into one issue. So a string, a number, `null`, and `undefined` all go through
 * untouched rather than being normalized or dropped.
 */
export function reportError(
  error: unknown,
  options: ReportOptions = {},
): string | undefined {
  const kind = options.kind ?? DEFAULT_KIND;

  // The second argument is a CaptureContext, which creates a temporary scope
  // with no mutation. `withScope` mutates and restores, which is unsafe under
  // concurrent async work.
  return Sentry.captureException(error, {
    level: options.level ?? DEFAULT_LEVEL,
    tags: resolveTags(kind, options),
    fingerprint: resolveFingerprint(kind, options),
    contexts: options.data ? { gofin: options.data } : undefined,
  });
}
