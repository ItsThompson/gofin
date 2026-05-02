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

/** Expense entry from the immutable ledger. */
export interface Expense {
  id: string;
  userId: string;
  name: string;
  /** Amount in minor units (cents). */
  amount: number;
  /** ISO 4217 currency code. */
  currency: string;
  /** One of: "essentials", "desires", "savings". */
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
  periodYear: number;
  periodMonth: number;
  status: string;
  correctsId?: string;
  isProRata: boolean;
  proRataGroup?: string;
  proRataIndex?: number;
  proRataTotal?: number;
  createdAt: string;
}

/** Response from POST /api/expenses. */
export interface ExpenseResponse {
  expense: Expense;
}

/** Request body for POST /api/expenses. */
export interface CreateExpenseRequest {
  name: string;
  /** Amount in minor units (cents). */
  amount: number;
  currency: string;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
  periodYear: number;
  periodMonth: number;
}

/** Request body for PUT /api/finance/defaults. */
export interface UpdateDefaultsRequest {
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  currency: string;
}

/** Request body for PUT /api/auth/me. */
export interface UpdateProfileRequest {
  username: string;
  email: string;
  currency: string;
}

/** Request body for POST /api/auth/me/password. */
export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
}

/** Auth response wrapping a User (used by profile update, password change). */
export interface AuthResponse {
  user: User;
}
