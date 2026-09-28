import type {
  BudgetPeriod,
  DefaultSettings,
  CreatePeriodRequest,
  PeriodSummary,
  TagSpending,
  CumulativeSpendPoint,
  Expense,
  HistoricalComparison,
  ProRataSchedule,
  TrendPoint,
  HealthScore,
  HealthScoreConfigureBudget,
  HealthScoreTrendPoint,
} from "@gofin/core";
import type { ActiveExpenseSuggestion } from "../expense-autocomplete/types";

export interface DashboardData {
  summary: PeriodSummary | null;
  tagSpending: TagSpending[];
  cumulativeData: CumulativeSpendPoint[];
  recentExpenses: Expense[];
  comparison: HistoricalComparison | null;
  upcomingProRata: ProRataSchedule[];
  trendData: TrendPoint[] | null;
  healthScore: HealthScore | HealthScoreConfigureBudget | null;
  healthScoreTrend: HealthScoreTrendPoint[] | null;
}

/** Base properties available in all period states. */
interface PeriodStateBase {
  /** Re-fetch period data. */
  retry: () => void;
}

/** Period is being fetched. */
export interface PeriodLoading extends PeriodStateBase {
  status: "loading";
}

/** No period exists for current month. User can create one. */
export interface PeriodNotFound extends PeriodStateBase {
  status: "no-period";
  /** Default budget settings for pre-filling the create form. Null when no settings exist. */
  defaults: DefaultSettings | null;
  /** Create a new period with given settings. */
  createPeriod: (body: CreatePeriodRequest) => void;
  /** Whether period creation is in progress. */
  creating: boolean;
  /** Error from last create attempt, or null. */
  createError: string | null;
  /** Clear the create error. */
  clearCreateError: () => void;
}

/** Period exists and is active. */
export interface PeriodActive extends PeriodStateBase {
  status: "active";
  /** The current budget period. Guaranteed non-null. */
  period: BudgetPeriod;
}

/** Fetch failed with a non-recoverable error. */
export interface PeriodError extends PeriodStateBase {
  status: "error";
}

/** Discriminated union of all possible period states. */
export type PeriodStateResult = PeriodLoading | PeriodNotFound | PeriodActive | PeriodError;

export interface SectionError {
  message: string;
}

export type SectionState<T> =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "success"; data: T }
  | { status: "empty" }
  | { status: "error"; error: SectionError };

export type BreakdownChart = "tag-spending" | "repeated-expenses";

export interface ExpenseSuggestionsState {
  status: "idle" | "loading" | "success" | "empty" | "error";
  suggestions: readonly ActiveExpenseSuggestion[];
  errorMessage: string | null;
}

export type DashboardSectionKey =
  | "summary"
  | "byTag"
  | "cumulative"
  | "recentExpenses"
  | "comparison"
  | "upcomingProRata"
  | "trends"
  | "healthScore"
  | "healthScoreTrend"
  | "suggestions";

export interface DashboardSectionState {
  summary: SectionState<PeriodSummary>;
  byTag: SectionState<readonly TagSpending[]>;
  cumulative: SectionState<readonly CumulativeSpendPoint[]>;
  recentExpenses: SectionState<readonly Expense[]>;
  comparison: SectionState<HistoricalComparison>;
  upcomingProRata: SectionState<readonly ProRataSchedule[]>;
  trends: SectionState<readonly TrendPoint[]>;
  healthScore: SectionState<HealthScore | HealthScoreConfigureBudget>;
  healthScoreTrend: SectionState<readonly HealthScoreTrendPoint[]>;
  suggestions: ExpenseSuggestionsState;
}

export type DashboardControllerStatus =
  | "active"
  | "loading"
  | "error"
  | "no-period"
  | "not-found";

export interface DashboardPeriodRecovery {
  defaults: DefaultSettings | null;
  createPeriod: (body: CreatePeriodRequest) => void;
  creating: boolean;
  createError: string | null;
  clearCreateError: () => void;
}

export interface DashboardDataResult {
  data: DashboardData;
  sections: DashboardSectionState;
  period: BudgetPeriod;
  periodStatus: DashboardControllerStatus;
  periodError: string | null;
  periodRecovery: DashboardPeriodRecovery | null;
  desktopVisible: boolean;
  loading: boolean;
  refresh: () => void;
  retry: (section: DashboardSectionKey) => void;
  replacePeriodAfterEdit: (period: BudgetPeriod) => void;
  trendMonths: 6 | 12;
  setTrendMonths: (months: 6 | 12) => void;
  breakdownChart: BreakdownChart;
  selectBreakdown: (chart: BreakdownChart) => void;
}
