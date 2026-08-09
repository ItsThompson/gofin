import type { SeverityLevel } from "@sentry/react-router";
import { ApiRequestError } from "../client";
import type { ErrorKind } from "./kinds";

/** The taxonomy fields decided by a failure's shape rather than by its call site. */
export interface FailureClassification {
  kind: ErrorKind;
  level: SeverityLevel;
  /**
   * Emits the tag `expected="true"`, which the client and server `beforeSend`
   * chains drop, so the event is visible in development and spends no quota.
   */
  expected: boolean;
}

/**
 * Browser transport failures. Callers pass this instead of calling
 * `classifyApiFailure`, because they have already run `isNetworkError` to choose
 * the user-facing message and the two readings must agree.
 *
 * Frozen: a shared literal that decides whether an event spends quota should not
 * be mutable by one of its consumers.
 */
export const NETWORK_FAILURE: Readonly<FailureClassification> = Object.freeze({
  kind: "network",
  level: "warning",
  expected: false,
});

/**
 * A sub-500 status is the highest-frequency failure class in this codebase and is
 * never a defect: an expired session, a rejected field, a refused permission.
 * Marking it expected is what keeps a routine session expiry from spending the
 * monthly error allowance.
 */
export function classifyApiFailure(error: unknown): FailureClassification {
  if (!(error instanceof ApiRequestError)) {
    return { kind: "internal", level: "error", expected: false };
  }

  if (error.status >= 500) {
    return { kind: "upstream", level: "error", expected: false };
  }

  return { kind: "validation", level: "warning", expected: true };
}
