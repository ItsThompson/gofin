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

/** Supported currencies with display metadata. */
export const SUPPORTED_CURRENCIES = [
  { code: "USD", symbol: "$", name: "US Dollar" },
  { code: "EUR", symbol: "€", name: "Euro" },
  { code: "GBP", symbol: "£", name: "British Pound" },
  { code: "JPY", symbol: "¥", name: "Japanese Yen" },
  { code: "CAD", symbol: "C$", name: "Canadian Dollar" },
  { code: "AUD", symbol: "A$", name: "Australian Dollar" },
  { code: "CHF", symbol: "CHF", name: "Swiss Franc" },
  { code: "CNY", symbol: "¥", name: "Chinese Yuan" },
  { code: "SGD", symbol: "S$", name: "Singapore Dollar" },
  { code: "HKD", symbol: "HK$", name: "Hong Kong Dollar" },
] as const;

/** Type for a supported currency entry. */
export type SupportedCurrency = (typeof SUPPORTED_CURRENCIES)[number];
