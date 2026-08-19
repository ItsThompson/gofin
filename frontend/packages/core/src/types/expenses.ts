import type { ExpenseType } from "../constants";

/** Lifecycle status of a ledger expense. */
export type ExpenseStatus = "active" | "corrected";

/** Expense entry from the immutable ledger. */
export interface Expense {
  id: string;
  userId: string;
  name: string;
  /** Amount in transaction currency minor units. */
  amount: number;
  /** Canonical transaction ISO 4217 currency code. */
  transactionCurrency: string;
  /** One of: "essentials", "desires", "savings". */
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
}

/** Request body for POST /api/expenses/:id/correct. */
export interface CorrectExpenseRequest {
  name: string;
  /** Amount in transaction currency minor units. */
  amount: number;
  expenseType: ExpenseType;
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
}

/** Response from GET /api/expenses/:id/history. */
export interface CorrectionHistoryResponse {
  entries: Expense[];
}
