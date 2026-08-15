/** Category breakdown within a period summary (E/D/S). */
export interface CategorySummary {
  /** Allocated amount in minor units (cents). */
  allocated: number;
  /** Spent amount in minor units (cents). */
  spent: number;
  /** Remaining = allocated - spent (can be negative). */
  remaining: number;
  /** Percentage of allocation used (0-100+). */
  percentUsed: number;
}

/** Full period summary returned by GET /api/finance/summary. */
export interface PeriodSummary {
  periodId: string;
  year: number;
  month: number;
  /** Budget total in minor units (cents). */
  totalBudget: number;
  /** Sum of all active expenses in minor units (cents). */
  totalSpent: number;
  /** totalBudget - totalSpent (can be negative). */
  remaining: number;
  daysInPeriod: number;
  daysElapsed: number;
  /** totalSpent / daysElapsed (cents per day). 0 if daysElapsed is 0. */
  dailySpendRate: number;
  /** remaining / daysRemaining (cents per day). 0 if no days remain. */
  budgetPace: number;
  /** True when dailySpendRate <= idealDailyRate. */
  isOnTrack: boolean;
  essentials: CategorySummary;
  desires: CategorySummary;
  savings: CategorySummary;
}

/** Response from GET /api/finance/summary. */
export interface SummaryResponse {
  summary: PeriodSummary;
}

/** Single tag's spending data for the tag spending chart. */
export interface TagSpending {
  tagId: string;
  tagName: string;
  /** Amount in minor units (cents). */
  amount: number;
  /** Percentage of total period spending. */
  percentOfTotal: number;
}

/** Response from GET /api/finance/spending/by-tag. */
export interface TagSpendingResponse {
  tagSpending: TagSpending[];
}

/** One data point in the cumulative spend chart. */
export interface CumulativeSpendPoint {
  /** Day of month (1-31). */
  day: number;
  /** Cumulative actual spending in minor units (cents). */
  actual: number;
  /** Ideal cumulative spending at this day (linear budget pace) in cents. */
  ideal: number;
}

/** Response from GET /api/finance/spending/cumulative. */
export interface CumulativeSpendResponse {
  points: CumulativeSpendPoint[];
}

/** Historical spending comparison data. */
export interface HistoricalComparison {
  /** Current period total spent in minor units (cents). */
  currentSpent: number;
  /** Previous period total spent in minor units (cents). */
  previousSpent: number;
  /** Reporting currency of the previous period. Empty when no previous period. */
  previousReportingCurrency: string;
  /** False when current and previous periods have different reporting currencies. */
  comparable: boolean;
  /** Rolling average of last 3 periods' totalSpent. Null if < 3 periods or mixed currencies. */
  rollingAverage: number | null;
  /** Percentage change from previous period. Only meaningful when comparable is true. */
  changePercent: number;
}

/** Response from GET /api/finance/spending/comparison. */
export interface HistoricalComparisonResponse {
  comparison: HistoricalComparison;
}
