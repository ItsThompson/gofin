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
  installmentAmountInMinorUnits: number;
  transactionCurrencyCode: string;
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
  totalAmountInMinorUnits: number;
  transactionCurrencyCode: string;
  expenseType: ExpenseType;
  periodYear: number;
  periodMonth: number;
  tagId: string;
  expenseDateIso: string;
  spreadOverMonths: number;
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
