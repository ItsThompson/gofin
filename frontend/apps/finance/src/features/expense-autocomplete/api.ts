import { apiClient } from "@gofin/api";
import type { ExpenseSuggestionsResponse } from "./types";

export const expenseSuggestionsApi = {
  getSuggestions: (page: number, pageSize: number, signal?: AbortSignal) =>
    apiClient<ExpenseSuggestionsResponse>(
      `/api/expenses/suggestions?page=${page}&pageSize=${pageSize}`,
      { signal },
    ),
};
