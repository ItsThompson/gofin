import { expenseSuggestionsApi } from "../../expense-autocomplete/api";
import {
  isActiveExpenseSuggestion,
  type ActiveExpenseSuggestion,
} from "../components/widgets/expenseFrecencyChartData";

export type DashboardSectionPayload = {
  section: "suggestions";
  data: readonly ActiveExpenseSuggestion[];
};

async function fetchSuggestions(
  pageSize: number,
  signal: AbortSignal,
  forceRefresh: boolean,
): Promise<readonly ActiveExpenseSuggestion[]> {
  const suggestions: ActiveExpenseSuggestion[] = [];
  let page = 1;
  let hasMore = true;
  while (suggestions.length < pageSize && hasMore && !signal.aborted) {
    const response = await expenseSuggestionsApi.getSuggestions(
      page,
      pageSize,
      {
        signal,
        forceRefresh,
      },
    );
    suggestions.push(...response.data.filter(isActiveExpenseSuggestion));
    hasMore = response.hasMore;
    page += 1;
  }
  return suggestions.slice(0, pageSize);
}

export async function fetchDashboardSection(
  signal: AbortSignal,
  forceRefresh = false,
): Promise<DashboardSectionPayload> {
  const suggestions = await fetchSuggestions(10, signal, forceRefresh);
  return { section: "suggestions", data: suggestions };
}
