import type { SeverityLevel } from "@sentry/react-router";
import type { ErrorKind } from "./kinds";

/** Options for a single call to `reportError`. */
export interface ReportOptions {
  /** Defaults to "error". */
  level?: SeverityLevel;

  /** Classification. Becomes the tag `error_kind`. Defaults to "internal". */
  kind?: ErrorKind;

  /** Logical operation, e.g. "expense.create". Becomes the tag `operation`. */
  op?: string;

  /** Business area, e.g. "expenses". Becomes the tag `domain`. */
  domain?: string;

  /**
   * Overrides the derived logical grouping key. The emitted fingerprint stays
   * ["{{ default }}", key], so this can only refine Sentry's own grouping.
   */
  groupKey?: string;

  /**
   * Replaces Sentry's grouping entirely with [groupKey], collapsing every
   * matching event into one issue. Only for a generic infrastructure failure
   * whose stack varies but whose meaning is singular. Ignored without a
   * `groupKey`, because an empty fingerprint would merge everything.
   */
  groupExact?: boolean;

  /** Additional low-cardinality tags. Values are stringified and truncated. */
  tags?: Record<string, string | number | boolean>;

  /**
   * Structured metadata, sent as the Sentry context block "gofin" rather than
   * as `extra`, which is deprecated in JS and absent from sentry-go. Sentry caps
   * a context block at 8 kB, so keep it small and flat. Field names, never
   * field values.
   */
  data?: Record<string, unknown>;

  /**
   * Marks the error as expected, so the client's `beforeSend` filter drops the
   * event and it never consumes error quota. The tag is emitted as the string
   * "true": that filter compares strictly, Sentry tag values are typed
   * `Primitive` so a boolean is legal, and nothing coerces it, so a boolean
   * value would silently stop matching.
   */
  expected?: boolean;
}
