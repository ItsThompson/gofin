import { apiClient } from "@gofin/api";
import type { ApiClientOptions } from "@gofin/api";
import type { ExpenseSuggestionsResponse } from "./types";

export const expenseSuggestionsApi = {
  getSuggestions: (
    page: number,
    pageSize: number,
    options?: Pick<ApiClientOptions, "forceRefresh" | "signal">,
  ) =>
    apiClient<ExpenseSuggestionsResponse>(
      `/api/expenses/suggestions?page=${page}&pageSize=${pageSize}`,
      options,
    ),
};
