export interface TrendPoint {
  year: number;
  month: number;
  /** Total spent in minor units (cents). */
  totalSpent: number;
  /** Budget amount in minor units (cents). */
  budgetAmount: number;
  /** Essentials category spent in minor units (cents). */
  essentialsSpent: number;
  /** Desires category spent in minor units (cents). */
  desiresSpent: number;
  /** Savings category spent in minor units (cents). */
  savingsSpent: number;
  /** Budgeted essentials percentage (0-100). */
  essentialsPercent: number;
  /** Budgeted desires percentage (0-100). */
  desiresPercent: number;
  /** Budgeted savings percentage (0-100). */
  savingsPercent: number;
  /** The period's reporting currency for this data point. */
  reportingCurrencyCode: string;
}

/** Response from GET /api/finance/spending/trends. */
export interface TrendResponse {
  trends: TrendPoint[];
}
