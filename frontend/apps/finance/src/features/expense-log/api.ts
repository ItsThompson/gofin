import { apiClient } from "@gofin/api";
import type { PaginatedResponse } from "@gofin/core";
import type {
  Expense,
  TagListResponse,
  PeriodListResponse,
} from "@gofin/core";

export const expenseLogApi = {
  getExpenses: (year: number, month: number) =>
    apiClient<PaginatedResponse<Expense>>(
      `/api/expenses?year=${year}&month=${month}&page=1&pageSize=1000`,
    ),

  getTags: () =>
    apiClient<TagListResponse>("/api/finance/tags").catch(
      (): TagListResponse => ({ tags: [] }),
    ),

  getPeriods: () =>
    apiClient<PeriodListResponse>("/api/finance/periods").catch(
      (): PeriodListResponse => ({ periods: [] }),
    ),
};
