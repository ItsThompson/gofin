import type { BudgetPeriod } from "@gofin/core";

/** Base properties available in all history row states. */
interface HistoricalPeriodRowBase {
  /** The budget period the row describes. */
  period: BudgetPeriod;
}

/** The period's summary loaded. A totalSpent of 0 means the user spent nothing. */
export interface LoadedPeriodRow extends HistoricalPeriodRowBase {
  status: "loaded";
  /** Amount spent in the period, in minor currency units. */
  totalSpent: number;
  /** budgetAmount minus totalSpent. Negative means a deficit. */
  surplus: number;
}

/** The period's summary fetch failed, so its spend is unknown. */
export interface UnavailablePeriodRow extends HistoricalPeriodRowBase {
  status: "unavailable";
}

/**
 * A history row is either loaded or unavailable. "Spent nothing" and "we do not
 * know what was spent" are different facts, so they are different states: an
 * optional totalSpent would let an unknown read as a zero.
 *
 * Any total computed across rows must exclude unavailable rows and state that it
 * is partial, because an unavailable row contributes an unknown amount, not 0.
 */
export type HistoricalPeriodRow = LoadedPeriodRow | UnavailablePeriodRow;

export interface HistoryDataResult {
  /** Historical period rows, each either loaded or unavailable. */
  periods: HistoricalPeriodRow[];
  /** Whether data is currently being fetched. */
  loading: boolean;
}
