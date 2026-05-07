import { apiClient } from "@gofin/api";
import type { PaginatedResponse } from "@gofin/core";
import type {
  Expense,
  ExpenseResponse,
  CorrectionHistoryResponse,
  CorrectExpenseRequest,
} from "../../types";

export const expenseDetailApi = {
  getExpense: (expenseId: string) =>
    apiClient<ExpenseResponse>(`/api/expenses/${expenseId}`),

  getCorrectionHistory: (expenseId: string) =>
    apiClient<CorrectionHistoryResponse>(`/api/expenses/${expenseId}/history`),

  getProRataGroup: (groupId: string) =>
    apiClient<PaginatedResponse<Expense>>(`/api/expenses/prorata/${groupId}`),

  submitCorrection: (expenseId: string, body: CorrectExpenseRequest) =>
    apiClient<ExpenseResponse>(`/api/expenses/${expenseId}/correct`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
};
