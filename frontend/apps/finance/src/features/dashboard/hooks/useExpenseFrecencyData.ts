import { useEffect, useState } from "react";
import { expenseSuggestionsApi } from "../../expense-autocomplete/api";
import {
  isActiveExpenseSuggestion,
  type ActiveExpenseSuggestion,
} from "../components/widgets/expenseFrecencyChartData";

export interface ExpenseFrecencyDataState {
  status: "loading" | "success" | "empty" | "error";
  suggestions: ActiveExpenseSuggestion[];
  errorMessage: string | null;
}

export interface UseExpenseFrecencyDataOptions {
  pageSize?: number;
}

const DEFAULT_PAGE_SIZE = 10;
const ERROR_MESSAGE = "Repeated expenses are unavailable right now.";

async function fetchActiveSuggestions(
  pageSize: number,
  signal: AbortSignal,
): Promise<ActiveExpenseSuggestion[]> {
  const suggestions: ActiveExpenseSuggestion[] = [];
  let page = 1;
  let hasMore = true;

  while (suggestions.length < pageSize && hasMore && !signal.aborted) {
    const response = await expenseSuggestionsApi.getSuggestions(page, pageSize, signal);
    suggestions.push(...response.data.filter(isActiveExpenseSuggestion));
    hasMore = response.hasMore;
    page += 1;
  }

  return suggestions.slice(0, pageSize);
}


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
        const suggestions = await fetchActiveSuggestions(pageSize, controller.signal);

        if (controller.signal.aborted) return;
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
