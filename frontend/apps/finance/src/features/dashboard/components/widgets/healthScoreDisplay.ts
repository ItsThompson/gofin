import type { HealthBand } from "../../../../types";

/**
 * Band color as a Tailwind text-color class. The ring reads it via
 * `fill="currentColor"` and the centered total inherits it, so one token drives
 * both. Reuses the default Tailwind palette (emerald/amber/red-500).
 */
export const BAND_COLOR_CLASS: Record<HealthBand, string> = {
  green: "text-emerald-500",
  amber: "text-amber-500",
  red: "text-red-500",
};

/** Short human label for each band (ticket wording). */
export const BAND_LABEL: Record<HealthBand, string> = {
  green: "On plan",
  amber: "Drifting",
  red: "Off plan",
};

/** Display label for the known sub-score keys. */
const COMPONENT_LABEL: Record<string, string> = {
  savings_achievement: "Savings",
  budget_adherence: "Budget adherence",
  allocation_balance: "Allocation balance",
  spending_stability: "Spending stability",
};

/**
 * Human label for a sub-score key. Falls back to a humanized form of the raw key
 * (snake_case -> Title Case) so an unknown or future component still renders a
 * sensible label instead of crashing. This is what keeps the card
 * backward-compatible with historical snapshots and forward-compatible with new
 * components the backend may add.
 */
export function componentLabel(key: string): string {
  return COMPONENT_LABEL[key] ?? humanizeKey(key);
}

function humanizeKey(key: string): string {
  return key
    .split("_")
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}
