import type { ProRataSchedule } from "./prorata";

/** Budget period for a single month. */
export interface BudgetPeriod {
  id: string;
  userId: string;
  year: number;
  month: number;
  /** Budget total in the reporting currency's minor units. */
  budgetAmount: number;
  reportingCurrencyCode: string;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  createdAt: string;
  updatedAt: string;
}

/** User's default budget settings. */
export interface DefaultSettings {
  userId: string;
  /** Budget amount in the default currency's minor units. 0 = not yet configured. */
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  currency: string;
  createdAt: string;
  updatedAt: string;
}

/** Response from GET /api/finance/defaults. */
export interface DefaultsResponse {
  defaults: DefaultSettings;
}

/** Response from GET/POST /api/finance/periods/*. */
export interface PeriodResponse {
  period: BudgetPeriod;
}

/** Response from GET /api/finance/periods. */
export interface PeriodListResponse {
  periods: BudgetPeriod[];
}

/** Request body for POST /api/finance/periods. */
export interface CreatePeriodRequest {
  year: number;
  month: number;
  /** Budget amount in the reporting currency's minor units. */
  budgetAmount: number;
  reportingCurrencyCode: string;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
}

/** Response from POST /api/finance/periods (with pro-rata). */
export interface CreatePeriodResponse {
  period: BudgetPeriod;
  appliedProRata: ProRataSchedule[];
  autoCreatedPeriods?: number;
  autoCreatedMonths?: string[];
}

/** Request body for PUT /api/finance/periods/:id. */
export interface UpdatePeriodRequest {
  /** Budget amount in the period reporting currency's minor units. */
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
}

/** Request body for PUT /api/finance/defaults. */
export interface UpdateDefaultsRequest {
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  currency: string;
}
