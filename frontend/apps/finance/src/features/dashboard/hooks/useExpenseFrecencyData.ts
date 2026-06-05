import { useEffect, useState } from "react";
import { expenseSuggestionsApi } from "../../expense-autocomplete/api";
import type { ExpenseSuggestion } from "../../expense-autocomplete/types";

export interface ExpenseFrecencyDataState {
  status: "loading" | "success" | "empty" | "error";
  suggestions: ExpenseSuggestion[];
  errorMessage: string | null;
}

export interface UseExpenseFrecencyDataOptions {
  pageSize?: number;
}

const DEFAULT_PAGE_SIZE = 10;
const ERROR_MESSAGE = "Repeated expenses are unavailable right now.";

export function useExpenseFrecencyData(
  options: UseExpenseFrecencyDataOptions = {},
): ExpenseFrecencyDataState {
  const pageSize = options.pageSize ?? DEFAULT_PAGE_SIZE;
  const [state, setState] = useState<ExpenseFrecencyDataState>({
    status: "loading",
    suggestions: [],
    errorMessage: null,
  });

  useEffect(() => {
    const controller = new AbortController();
    setState({ status: "loading", suggestions: [], errorMessage: null });

    async function fetchSuggestions() {
      try {
        const response = await expenseSuggestionsApi.getSuggestions(
          1,
          pageSize,
          controller.signal,
        );

        if (controller.signal.aborted) return;

        const suggestions = response.data.slice(0, pageSize);
        setState({
          status: suggestions.length > 0 ? "success" : "empty",
          suggestions,
          errorMessage: null,
        });
      } catch (error) {
        if (controller.signal.aborted) return;
        setState({
          status: "error",
          suggestions: [],
          errorMessage: error instanceof Error ? error.message : ERROR_MESSAGE,
        });
      }
    }

    void fetchSuggestions();

    return () => controller.abort();
  }, [pageSize]);

  return state;
}
