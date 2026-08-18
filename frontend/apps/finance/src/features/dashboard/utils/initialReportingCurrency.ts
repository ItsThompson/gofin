import type { DefaultSettings, User } from "@gofin/core";
import { isSupportedCurrency } from "@gofin/core";

/**
 * Picks the initial reporting currency for the create-period prompt from the
 * defaults, then the user profile. Returns "" when neither supplies a
 * supported currency, so the prompt starts with no selection instead of
 * assuming a fallback.
 */
export function initialReportingCurrency(
  defaults: DefaultSettings | null,
  user: User,
): string {
  for (const raw of [defaults?.currency, user.currency]) {
    const candidate = raw?.trim().toUpperCase() ?? "";
    if (candidate && isSupportedCurrency(candidate)) {
      return candidate;
    }
  }
  return "";
}
