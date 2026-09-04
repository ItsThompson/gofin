/** Color band for the health-score total. */
export type HealthBand = "green" | "amber" | "red";

/**
 * Known contributing sub-score keys. Mirrors the backend model constants. The
 * key on {@link HealthComponent} is an OPEN union, so a future or historical
 * component the current build has never seen is still representable.
 */
export type HealthComponentKey =
  | "savings_achievement"
  | "budget_adherence"
  | "allocation_balance"
  | "spending_stability";

/** Score is in [0, max]; max is the weight. */
export interface HealthComponent {
  /**
   * Component key. Open union: the backend may add or rename components across
   * formula versions, and historical snapshots may carry a key this build has
   * never seen. `componentLabel()` humanizes any unknown key.
   */
  key: HealthComponentKey | (string & {});
  /** Points earned (0-max). */
  score: number;
  /** Full-mark points for this component. */
  max: number;
  /** Plain-English one-liner, e.g. "Saved $420 of $600 target". */
  detail: string;
}

/** Rules-based plain-English read. Driver is empty when all components are maxed. */
export interface HealthInsight {
  summary: string;
  driver: HealthComponentKey | "";
  nudge: string;
}

/** Returned by GET /api/finance/health-score. */
export interface HealthScore {
  year: number;
  month: number;
  /** Sum of the present component scores, 0-100. */
  total: number;
  band: HealthBand;
  /** True for the current (open) month; firms up when the month closes. */
  provisional: boolean;
  formulaVersion: number;
  /** The scored period's immutable reporting currency. */
  reportingCurrency: string;
  components: HealthComponent[];
  insight: HealthInsight;
}

/** Returned instead of a score when the period has no budget configured. */
export interface HealthScoreConfigureBudget {
  configureBudget: true;
}

/**
 * Response from GET /api/finance/health-score. A discriminated union: either a
 * full score or the configure-budget prompt.
 */
export interface HealthScoreResponse {
  healthScore: HealthScore | HealthScoreConfigureBudget;
}

/** One month in the health-score trend sparkline. */
export interface HealthScoreTrendPoint {
  year: number;
  month: number;
  total: number;
  band: HealthBand;
  provisional: boolean;
  formulaVersion: number;
  /** The scored period's immutable reporting currency. */
  reportingCurrency: string;
}

/** Response from GET /api/finance/health-score/trend. */
export interface HealthScoreTrendResponse {
  trends: HealthScoreTrendPoint[];
}
