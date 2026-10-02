import type { TagSpending } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { fetchDashboardSection } from "../../hooks/dashboardDataRequests";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import type { BreakdownChart, ExpenseSuggestionsState } from "../../types";
import { BreakdownSection } from "../BreakdownSection";
import type { DashboardDesktopSectionProps } from "./types";

interface BreakdownDashboardSectionProps extends DashboardDesktopSectionProps {
  selectedChart: BreakdownChart;
  onSelectedChartChange: (chart: BreakdownChart) => void;
}

export function BreakdownDashboardSection({
  period,
  enabled,
  refreshVersion,
  currency,
  desktopVisible,
  selectedChart,
  onSelectedChartChange,
}: BreakdownDashboardSectionProps) {
  const tagSpending = useDashboardRequest<readonly TagSpending[]>(
    (signal, forceRefresh) =>
      dashboardApi
        .getTagSpending(period.year, period.month, { signal, forceRefresh })
        .then((response) => response.tagSpending),
    {
      enabled: enabled && desktopVisible,
      refreshVersion,
      operation: "dashboard.byTag",
      emptyWhen: (data) => data.length === 0,
      forceRefreshOnVersionChange: true,
    },
  );
  const suggestionsRequest = useDashboardRequest(
    (signal, forceRefresh) =>
      fetchDashboardSection(signal, forceRefresh).then((result) => {
        if (result.section !== "suggestions")
          throw new Error(
            "Dashboard suggestions response did not match the requested section.",
          );
        return result.data;
      }),
    {
      enabled:
        enabled && desktopVisible && selectedChart === "repeated-expenses",
      refreshVersion,
      operation: "dashboard.suggestions",
      emptyWhen: (data) => data.length === 0,
      forceRefreshOnVersionChange: true,
    },
  );
  const suggestionsState = suggestionsRequest.state;
  let suggestions: ExpenseSuggestionsState;
  switch (suggestionsState.status) {
    case "success":
      suggestions = {
        status: "success",
        suggestions: suggestionsState.data,
        errorMessage: null,
      };
      break;
    case "empty":
      suggestions = { status: "empty", suggestions: [], errorMessage: null };
      break;
    case "error":
      suggestions = {
        status: "error",
        suggestions: [],
        errorMessage: suggestionsState.error.message,
      };
      break;
    default:
      suggestions = {
        status: suggestionsState.status,
        suggestions: [],
        errorMessage: null,
      };
  }
  const handleChartChange = (chart: BreakdownChart) => {
    onSelectedChartChange(chart);
    if (chart === "repeated-expenses") suggestionsRequest.retry();
  };
  if (!desktopVisible) return null;
  return (
    <section id="breakdown" data-outline-title="Breakdown">
      <SectionErrorBoundary sectionName="Breakdown">
        <BreakdownSection
          tagSpending={
            tagSpending.state.status === "success"
              ? [...tagSpending.state.data]
              : []
          }
          tagSpendingState={tagSpending.state}
          expenseFrecencyData={suggestions}
          currency={currency}
          selectedChart={selectedChart}
          onChartChange={handleChartChange}
          onSuggestionsRetry={suggestionsRequest.retry}
          onTagSpendingRetry={tagSpending.retry}
        />
      </SectionErrorBoundary>
    </section>
  );
}
