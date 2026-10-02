import type { Expense } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { SectionState } from "../SectionState";
import { RecentExpenses } from "../widgets/RecentExpenses";
import { EmptyExpenses } from "./EmptyExpenses";
import type { DashboardSectionProps } from "./types";

interface RecentExpensesProps extends DashboardSectionProps {
  readOnly: boolean;
}

export function RecentExpensesDashboardSection({ period, enabled, refreshVersion, currency, readOnly }: RecentExpensesProps) {
  const expenses = useDashboardRequest<readonly Expense[]>(
    (signal, forceRefresh) => dashboardApi.getRecentExpenses(period.year, period.month, 5, { signal, forceRefresh }).then((response) => response.data),
    { enabled, refreshVersion, operation: "dashboard.recentExpenses", emptyWhen: (data) => data.length === 0, forceRefreshOnVersionChange: true },
  );

  return (
    <section id="recent-expenses" data-outline-title="Recent Expenses">
      <SectionState label="Recent expenses" state={expenses.state} onRetry={expenses.retry} emptyMessage={readOnly ? "No expenses recorded for this period." : "No expenses yet."} emptyContent={!readOnly ? <EmptyExpenses readOnly={false} /> : undefined}>
        {(data) => data.length === 0 ? <EmptyExpenses readOnly={readOnly} /> : <SectionErrorBoundary sectionName="Recent Expenses"><RecentExpenses expenses={[...data]} currency={currency} /></SectionErrorBoundary>}
      </SectionState>
    </section>
  );
}
