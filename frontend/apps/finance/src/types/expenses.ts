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
  status: "active" | "corrected";
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

/** Request body for POST /api/expenses/:id/correct. */
export interface CorrectExpenseRequest {
  name: string;
  /** Amount in minor units (cents). */
  amount: number;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
}

/** Response from GET /api/expenses/:id/history. */
export interface CorrectionHistoryResponse {
  entries: Expense[];
}
