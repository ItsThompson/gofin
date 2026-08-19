import type { ExpenseType } from "../constants";
import type { Expense } from "./expenses";

/** Lifecycle status of a pro-rata schedule installment. */
export type ProRataStatus = "pending" | "applied";

/** Pro-rata schedule record from the finance service. */
export interface ProRataSchedule {
  id: string;
  userId: string;
  proRataGroup: string;
  name: string;
  /** Installment amount in minor units (cents). */
  amount: number;
  transactionCurrency: string;
  expenseType: ExpenseType;
  tagId: string;
  targetYear: number;
  targetMonth: number;
  installmentIndex: number;
  installmentTotal: number;
  status: ProRataStatus;
  createdAt: string;
  appliedAt: string | null;
}

/** Request body for POST /api/finance/prorata. */
export interface CreateProRataRequest {
  name: string;
  /** Total amount in minor units (cents). */
  totalAmount: number;
  transactionCurrency: string;
  expenseType: ExpenseType;
  periodYear: number;
  periodMonth: number;
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
  /** Number of months to spread over (minimum 2). */
  months: number;
}

/** Response from POST /api/finance/prorata. */
export interface ProRataResponse {
  expense: Expense;
  schedules: ProRataSchedule[];
}

/** Response from GET /api/finance/prorata/upcoming. */
export interface UpcomingProRataResponse {
  schedules: ProRataSchedule[];
}
