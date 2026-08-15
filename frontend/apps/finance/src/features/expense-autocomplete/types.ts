import type { ExpenseType } from "@gofin/core";

export interface ExpenseSuggestionPatch {
  name: string;
  amountDollars: string;
  currency: string;
  expenseType: ExpenseType;
  tagId: string | null;
}

export interface ExpenseSuggestion {
  name: string;
  /** Original transaction amount in minor units from the latest active matching expense. */
  transactionAmount: number;
  /** Original transaction currency from the latest active matching expense. */
  transactionCurrency: string;
  /** Deprecated: mirrors transactionAmount for rollout compatibility. */
  amount: number;
  /** Deprecated: mirrors transactionCurrency for rollout compatibility. */
  currency: string;
  expenseType: ExpenseType;
  tagId: string;
  frequency: number;
  lastUsedAt: string;
  recencyBucket: "today" | "last_7_days" | "last_30_days" | "older";
  frecencyScore: number;
}

export interface ExpenseSuggestionsResponse {
  data: ExpenseSuggestion[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

export interface ExpenseAutocompleteState {
  candidates: ExpenseSuggestion[];
  visibleSuggestions: ExpenseSuggestion[];
  page: number;
  isInitialLoading: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  error: string | null;
}

export interface ExpenseAutocompleteActions {
  setQuery: (query: string) => void;
  loadMore: () => Promise<void>;
  clearError: () => void;
}
