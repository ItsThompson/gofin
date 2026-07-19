import type { HealthBand, HealthComponentKey } from "../../../../types";

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

/** Display label for each sub-score row. */
export const COMPONENT_LABEL: Record<HealthComponentKey, string> = {
  savings_achievement: "Savings",
  budget_adherence: "Budget adherence",
  allocation_balance: "Allocation balance",
};
