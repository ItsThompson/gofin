import type { BudgetPeriod, DefaultSettings, CreatePeriodRequest, CreatePeriodResponse } from "../../types";

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
  /** Default budget settings for pre-filling the create form. Null if defaults fetch failed. */
  defaults: DefaultSettings | null;
  /** Create a new period with given settings. */
  createPeriod: (body: CreatePeriodRequest) => void;
  /** Whether period creation is in progress. */
  creating: boolean;
  /** Error from last create attempt, or null. */
  createError: string | null;
  /** Clear the create error. */
  clearCreateError: () => void;
  /** Response from last successful creation (for pro-rata info display). */
  lastCreateResponse: CreatePeriodResponse | null;
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
