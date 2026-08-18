/** Valid expense type categories. */
export const EXPENSE_TYPES = ["essentials", "desires", "savings"] as const;

/** Derived type from EXPENSE_TYPES constant. */
export type ExpenseType = (typeof EXPENSE_TYPES)[number];

/** Default budget split percentages (must sum to 100). */
export const DEFAULT_BUDGET_SPLIT = {
  essentials: 50,
  desires: 30,
  savings: 20,
} as const;
