export { apiClient, ApiRequestError, consumeReturnToPath } from "./api-client";
export { formatCurrency, getCurrencySymbol } from "./currency";

/** Core user model returned by the auth API. */
export interface User {
  id: string;
  username: string;
  email: string;
  role: "user" | "admin";
  currency: string;
  hasCompletedOnboarding: boolean;
  createdAt: string;
}

/** API error response shape. All API errors follow this contract. */
export interface ApiError {
  /** Machine-readable error code (e.g., "VALIDATION_ERROR", "NOT_FOUND"). */
  code: string;
  /** Human-readable error message for display. */
  message: string;
  /** Optional field-level validation errors. */
  fields?: Record<string, string>;
}

/** Wrapper for paginated API responses. */
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

/** Budget period for a single month. */
export interface BudgetPeriod {
  id: string;
  userId: string;
  year: number;
  month: number;
  /** Budget total in minor units (cents). */
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  createdAt: string;
  updatedAt: string;
}

/** User's default budget settings. */
export interface DefaultSettings {
  userId: string;
  /** Budget amount in minor units (cents). 0 = not yet configured. */
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

/** Request body for POST /api/finance/periods. */
export interface CreatePeriodRequest {
  year: number;
  month: number;
  /** Budget amount in minor units (cents). */
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
}
