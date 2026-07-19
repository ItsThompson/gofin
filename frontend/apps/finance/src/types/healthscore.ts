/** Color band for the health-score total. */
export type HealthBand = "green" | "amber" | "red";

/** Contributing sub-score keys. Mirrors the backend model constants. */
export type HealthComponentKey =
  | "savings_achievement"
  | "budget_adherence"
  | "allocation_balance";

/** One contributing sub-score. Score is in [0, max]; max is the weight. */
export interface HealthComponent {
  key: HealthComponentKey;
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

/** Monthly financial health score from GET /api/finance/health-score. */
export interface HealthScore {
  year: number;
  month: number;
  /** Sum of the present component scores, 0-100. */
  total: number;
  band: HealthBand;
  /** True for the current (open) month; firms up when the month closes. */
  provisional: boolean;
  formulaVersion: number;
  components: HealthComponent[];
  insight: HealthInsight;
  /** True when the period has no budget configured; numeric fields are unset. */
  configureBudget?: boolean;
}

/** Response from GET /api/finance/health-score. */
export interface HealthScoreResponse {
  healthScore: HealthScore;
}
