import { useRef, useState } from "react";
import { Link } from "react-router";
import type { BudgetPeriod, User } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { Card, CardContent, CardHeader, CardTitle } from "@gofin/ui/components/card";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";
import { LayoutDashboard, PlusCircle, Settings2, Wallet, RefreshCw } from "lucide-react";
import { useDashboardData } from "../hooks/useDashboardData";
import { BudgetSettingsEditor } from "./BudgetSettingsEditor";
import { CreatePeriodPrompt } from "./CreatePeriodPrompt";
import { TrendsSection } from "./TrendsSection";
import { BreakdownSection } from "./BreakdownSection";import { DashboardOutline } from "./DashboardOutline";
import { SectionState } from "./SectionState";
import { SummaryBar } from "./widgets/SummaryBar";
import { CategoryGauges } from "./widgets/CategoryGauges";
import { PacingIndicator } from "./widgets/PacingIndicator";
import { CumulativeSpendChart } from "./widgets/CumulativeSpendChart";
import { RecentExpenses } from "./widgets/RecentExpenses";
import { HistoricalComparisonWidget } from "./widgets/HistoricalComparisonWidget";
import { UpcomingProRataSection } from "./widgets/UpcomingProRataSection";
import { HealthScoreCard } from "./widgets/HealthScoreCard";

export interface ActiveDashboardProps {
  period: BudgetPeriod;
  user?: User;
  readOnly?: boolean;
}

export function ActiveDashboard({ period, user, readOnly = false }: ActiveDashboardProps) {
  const [showSettings, setShowSettings] = useState(false);
  const dashboardContentRef = useRef<HTMLDivElement | null>(null);
  const controller = useDashboardData(period, readOnly);
  const { sections, period: renderedPeriod } = controller;
  const breakdownChart = controller.breakdownChart;
  const currency = renderedPeriod.reportingCurrencyCode;
  const monthName = new Date(renderedPeriod.year, renderedPeriod.month - 1).toLocaleString("en-US", {
    month: "long",
    year: "numeric",
  });

  if (controller.periodStatus === "loading") return <DashboardSkeleton />;
  if (controller.periodStatus === "no-period") {
    if (!user || !controller.periodRecovery) {
      return <Card role="alert"><CardContent>Could not restore this budget period.</CardContent></Card>;
    }
    return (
      <CreatePeriodPrompt
        defaults={controller.periodRecovery.defaults}
        user={user}
        year={period.year}
        month={period.month}
        onCreatePeriod={controller.periodRecovery.createPeriod}
        creating={controller.periodRecovery.creating}
        createError={controller.periodRecovery.createError}
      />
    );
  }
  if (controller.periodStatus === "not-found") {
    return (
      <Card role="alert">
        <CardHeader><CardTitle className="text-destructive">Historical period not found</CardTitle></CardHeader>
        <CardContent><p className="text-sm text-muted-foreground">{controller.periodError}</p></CardContent>
      </Card>
    );
  }
  if (controller.periodStatus === "error") {
    return (
      <Card role="alert">
        <CardHeader><CardTitle className="text-destructive">Could not load the dashboard</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">{controller.periodError}</p>
          <Button variant="outline" onClick={controller.refresh}>Retry</Button>
        </CardContent>
      </Card>
    );
  }

  function handlePeriodUpdated(updatedPeriod: BudgetPeriod) {
    setShowSettings(false);
    controller.replacePeriodAfterEdit(updatedPeriod);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <LayoutDashboard className="size-6 text-primary" />
          <h1 className="text-2xl font-bold">{readOnly ? monthName : "Dashboard"}</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={controller.refresh} aria-label="Refresh all data">
            <RefreshCw className="size-4" />
            <span className="hidden sm:inline ml-1">Refresh all data</span>
          </Button>
          {!readOnly && <>
            <Button variant="outline" size="sm" onClick={() => setShowSettings((visible) => !visible)} aria-label="Budget Settings">
              <Settings2 className="size-4" />
              <span className="hidden sm:inline ml-1">Budget Settings</span>
            </Button>
            <Button asChild className="hidden md:inline-flex">
              <Link to="/expenses/new"><PlusCircle className="size-4" />Log Expense</Link>
            </Button>
          </>}
        </div>
      </div>

      {showSettings && !readOnly && <SectionErrorBoundary sectionName="Budget Settings">
        <BudgetSettingsEditor period={renderedPeriod} onSaved={handlePeriodUpdated} onCancel={() => setShowSettings(false)} />
      </SectionErrorBoundary>}

      <div ref={dashboardContentRef} className="space-y-6">
        <section id="health-score" data-outline-title="Health Score">
          <SectionState label="Health Score" state={sections.healthScore} onRetry={() => controller.retry("healthScore")} emptyMessage="No health score is available for this period.">
            {(score) => <SectionErrorBoundary sectionName="Health Score"><HealthScoreCard score={score} trend={sections.healthScoreTrend.status === "success" ? [...sections.healthScoreTrend.data] : null} /></SectionErrorBoundary>}
          </SectionState>
          <SectionState
            label="Health score trend"
            state={sections.healthScoreTrend}
            onRetry={() => controller.retry("healthScoreTrend")}
            emptyMessage="No health score trend is available."
          >
            {() => null}
          </SectionState>
        </section>

        <section id="summary" data-outline-title="Summary" className="space-y-6">
          <SectionState label="Summary" state={sections.summary} onRetry={() => controller.retry("summary")} emptyMessage="No summary data is available for this period.">
            {(summary) => <>
              <SectionErrorBoundary sectionName="Summary"><SummaryBar budgetAmount={renderedPeriod.budgetAmount} totalSpent={summary.totalSpent} remaining={summary.remaining} daysLeft={summary.daysInPeriod - summary.daysElapsed} currency={currency} /></SectionErrorBoundary>
              <section id="budget-allocations" data-outline-title="Budget Allocations"><SectionErrorBoundary sectionName="Category Gauges"><CategoryGauges summary={summary} currency={currency} /></SectionErrorBoundary></section>
              <div className="hidden md:grid md:grid-cols-2 md:gap-6">
                <section id="spending-pace" data-outline-title="Spending Pace"><SectionErrorBoundary sectionName="Spending Pace"><PacingIndicator summary={summary} currency={currency} /></SectionErrorBoundary></section>
              </div>
            </>}
          </SectionState>
        </section>

        <section id="upcoming-prorata" data-outline-title="Upcoming Pro-rata">
          <SectionState label="Upcoming pro-rata" state={sections.upcomingProRata} onRetry={() => controller.retry("upcomingProRata")} emptyMessage="No upcoming pro-rata payments.">
            {(schedules) => <SectionErrorBoundary sectionName="Upcoming Pro-rata"><UpcomingProRataSection schedules={[...schedules]} currency={currency} /></SectionErrorBoundary>}
          </SectionState>
        </section>

        <div className="hidden md:block space-y-6">
          {controller.desktopVisible && <>
          {sections.comparison.status === "success" ? <section id="historical-comparison" data-outline-title="Historical Comparison">
            <SectionState label="Historical comparison" state={sections.comparison} onRetry={() => controller.retry("comparison")} emptyMessage="Not enough data for comparison.">
              {(comparison) => <SectionErrorBoundary sectionName="Historical Comparison"><HistoricalComparisonWidget comparison={comparison} currency={currency} /></SectionErrorBoundary>}
            </SectionState>
          </section> : <SectionState label="Historical comparison" state={sections.comparison} onRetry={() => controller.retry("comparison")} emptyMessage="Not enough data for comparison.">{() => null}</SectionState>}
          {sections.trends.status === "success" ? <section id="trends" data-outline-title="Trends">
            <SectionState label="Trends" state={sections.trends} onRetry={() => controller.retry("trends")} emptyMessage="No trend data is available.">
              {(trendData) => <SectionErrorBoundary sectionName="Monthly Trends"><TrendsSection trendData={[...trendData]} trendMonths={controller.trendMonths} onToggle={controller.setTrendMonths} currency={currency} /></SectionErrorBoundary>}
            </SectionState>
          </section> : <SectionState label="Trends" state={sections.trends} onRetry={() => controller.retry("trends")} emptyMessage="No trend data is available.">{() => null}</SectionState>}
          {sections.byTag.status === "success" || sections.byTag.status === "empty" ? <section id="breakdown" data-outline-title="Breakdown">
            <SectionState label="Breakdown" state={sections.byTag} onRetry={() => controller.retry("byTag")} emptyMessage="No tag spending is available." emptyContent={<BreakdownSection tagSpending={[]} expenseFrecencyData={sections.suggestions} currency={currency} selectedChart={breakdownChart} onChartChange={controller.selectBreakdown} onSuggestionsRetry={() => controller.retry("suggestions")} />}>
              {(tagSpending) => <SectionErrorBoundary sectionName="Breakdown"><BreakdownSection tagSpending={[...tagSpending]} expenseFrecencyData={sections.suggestions} currency={currency} selectedChart={breakdownChart} onChartChange={controller.selectBreakdown} onSuggestionsRetry={() => controller.retry("suggestions")} /></SectionErrorBoundary>}
            </SectionState>
          </section> : <SectionState label="Breakdown" state={sections.byTag} onRetry={() => controller.retry("byTag")} emptyMessage="No tag spending is available.">{() => null}</SectionState>}
          {sections.cumulative.status === "success" ? <section id="cumulative-spending" data-outline-title="Cumulative Spending">
            <SectionState label="Cumulative spending" state={sections.cumulative} onRetry={() => controller.retry("cumulative")} emptyMessage="No cumulative spending data is available.">
              {(points) => <SectionErrorBoundary sectionName="Cumulative Spending"><CumulativeSpendChart data={[...points]} currency={currency} /></SectionErrorBoundary>}
            </SectionState>
          </section> : <SectionState label="Cumulative spending" state={sections.cumulative} onRetry={() => controller.retry("cumulative")} emptyMessage="No cumulative spending data is available.">{() => null}</SectionState>}
          </>}
        </div>

        <section id="recent-expenses" data-outline-title="Recent Expenses">
          <SectionState label="Recent expenses" state={sections.recentExpenses} onRetry={() => controller.retry("recentExpenses")} emptyMessage={readOnly ? "No expenses recorded for this period." : "No expenses yet."} emptyContent={!readOnly ? <Card><CardContent className="flex flex-col items-center justify-center py-12 text-center"><Wallet className="mb-4 size-12 text-muted-foreground/50" /><h2 className="mb-2 text-lg font-semibold">No expenses yet</h2><p className="mb-6 max-w-sm text-sm text-muted-foreground">Start tracking your spending by logging your first expense for this month.</p><Button asChild><Link to="/expenses/new"><PlusCircle className="size-4" />Log your first expense</Link></Button></CardContent></Card> : undefined}>
            {(expenses) => expenses.length === 0
              ? readOnly
                ? <Card><CardContent className="py-8 text-center text-sm text-muted-foreground">No expenses recorded for this period.</CardContent></Card>
                : <Card><CardContent className="flex flex-col items-center justify-center py-12 text-center"><Wallet className="mb-4 size-12 text-muted-foreground/50" /><h2 className="mb-2 text-lg font-semibold">No expenses yet</h2><p className="mb-6 max-w-sm text-sm text-muted-foreground">Start tracking your spending by logging your first expense for this month.</p><Button asChild><Link to="/expenses/new"><PlusCircle className="size-4" />Log your first expense</Link></Button></CardContent></Card>
              : <SectionErrorBoundary sectionName="Recent Expenses"><RecentExpenses expenses={[...expenses]} currency={currency} /></SectionErrorBoundary>}
          </SectionState>
        </section>
      </div>
      <DashboardOutline rootRef={dashboardContentRef} />
    </div>
  );
}
