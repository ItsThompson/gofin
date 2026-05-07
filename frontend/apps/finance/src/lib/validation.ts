/**
 * Validates that E/D/S (Essentials/Desires/Savings) percentages are valid.
 *
 * Rules:
 * - All values must be non-negative
 * - Values must sum to exactly 100
 *
 * Returns an error message string if invalid, or null if valid.
 *
 * NOTE: This is the canonical validation function. The DashboardPage's
 * CreatePeriodPrompt uses equivalent logic inline. The SettingsPage has its
 * own inline `validateEDSSplit` that only checks the sum (missing the
 * non-negative check). Ticket #15 will unify both pages to use this shared
 * function.
 */
export function validateEDSSplit(
  essentials: number,
  desires: number,
  savings: number,
): string | null {
  if (essentials < 0 || desires < 0 || savings < 0) {
    return "Percentages must be non-negative";
  }

  const total = essentials + desires + savings;
  if (total !== 100) {
    return `Percentages must sum to 100% (currently ${total}%)`;
  }

  return null;
}
