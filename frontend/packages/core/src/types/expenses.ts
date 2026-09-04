import type { ExpenseType } from "../constants";

export type ExpenseStatus = "active" | "corrected";

/** Expense entry from the immutable ledger. */
export interface Expense {
  id: string;
  userId: string;
  name: string;
  /** Canonical transaction ISO 4217 currency code. */
  transactionCurrency: string;
  expenseType: ExpenseType;
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
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
  /** Original amount in transaction currency minor units. */
  transactionAmount: number;
  /** Converted amount in the period reporting currency minor units. */
  reportingAmount: number;
  reportingCurrency: string;
  /** Source-to-target exchange rate. */
  exchangeRate?: string;
  /** "open_exchange_rates" | "identity" | "migration". */
  exchangeRateSource?: string;
  exchangeRateTimestamp?: string;
  /** Present for live provider snapshots with cache expiry metadata. */
  exchangeRateExpiresAt?: string;
  clientGeneratedIdempotencyKey?: string;
}

/** Response from POST /api/expenses. */
export interface ExpenseResponse {
  expense: Expense;
}

/** Request body for POST /api/expenses. */
export interface CreateExpenseRequest {
  name: string;
  /** Amount in transaction currency minor units. */
  amount: number;
  transactionCurrency: string;
  expenseType: ExpenseType;
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
  periodYear: number;
  periodMonth: number;
  clientGeneratedIdempotencyKey?: string;
}

/** Request body for POST /api/expenses/:id/correct. */
export interface CorrectExpenseRequest {
  name: string;
  /** Amount in transaction currency minor units. */
  amount: number;
  /** Canonical transaction currency for the correction. */
  transactionCurrency?: string;
  expenseType: ExpenseType;
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
}

/** Response from GET /api/expenses/:id/history. */
export interface CorrectionHistoryResponse {
  entries: Expense[];
}
