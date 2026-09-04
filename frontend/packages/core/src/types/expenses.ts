import type { ExpenseType } from "../constants";

export type ExpenseStatus = "active" | "corrected";

/** Expense entry from the immutable ledger. */
export interface Expense {
  id: string;
  userId: string;
  name: string;
  transactionCurrencyCode: string;
  expenseType: ExpenseType;
  tagId: string;
  expenseDateIso: string;
  periodYear: number;
  periodMonth: number;
  status: ExpenseStatus;
  correctsId?: string;
  isProRata: boolean;
  proRataGroup?: string;
  proRataIndex?: number;
  proRataTotal?: number;
  createdAt: string;
  // Money snapshot fields (backfilled for every row)
  originalTransactionAmountInMinorUnits: number;
  reportingAmountInMinorUnits: number;
  reportingCurrencyCode: string;
  sourceToTargetExchangeRate?: string;
  exchangeRateSource?: ExchangeRateSource;
  exchangeRateTimestamp?: string;
  /** Present for live provider snapshots with cache expiry metadata. */
  exchangeRateCacheExpiresAt?: string;
  clientGeneratedIdempotencyKey?: string;
}

export type ExchangeRateSource = "open_exchange_rates" | "identity" | "migration";

/** Response from POST /api/expenses. */
export interface ExpenseResponse {
  expense: Expense;
}

/** Request body for POST /api/expenses. */
export interface CreateExpenseRequest {
  name: string;
  amountInTransactionCurrencyMinorUnits: number;
  transactionCurrencyCode: string;
  expenseType: ExpenseType;
  tagId: string;
  expenseDateIso: string;
  periodYear: number;
  periodMonth: number;
  clientGeneratedIdempotencyKey: string;
}

/** Request body for POST /api/expenses/:id/correct. */
export interface CorrectExpenseRequest {
  name: string;
  amountInTransactionCurrencyMinorUnits: number;
  transactionCurrencyCode?: string;
  expenseType: ExpenseType;
  tagId: string;
  expenseDateIso: string;
}

/** Response from GET /api/expenses/:id/history. */
export interface CorrectionHistoryResponse {
  entries: Expense[];
}
