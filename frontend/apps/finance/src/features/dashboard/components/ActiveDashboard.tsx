import { useState } from "react";
import { Link } from "react-router";
import type { User } from "@gofin/core";
import type { BudgetPeriod } from "../../../types";
import { Button } from "@gofin/ui/components/button";
import {
  Card,
  CardContent,
} from "@gofin/ui/components/card";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";
import {
  LayoutDashboard,
  PlusCircle,
  Wallet,
  Settings2,
} from "lucide-react";
import { useDashboardData } from "../hooks/useDashboardData";
import { BudgetSettingsEditor } from "./BudgetSettingsEditor";
import { MonthlyTrendsSection } from "./MonthlyTrendsSection";
import { SummaryBar } from "./widgets/SummaryBar";
import { CategoryGauges } from "./widgets/CategoryGauges";
import { PacingIndicator } from "./widgets/PacingIndicator";
import { TagSpendingChart } from "./widgets/TagSpendingChart";
import { CumulativeSpendChart } from "./widgets/CumulativeSpendChart";
import { RecentExpenses } from "./widgets/RecentExpenses";
import { HistoricalComparisonWidget } from "./widgets/HistoricalComparisonWidget";
import { UpcomingProRataSection } from "./widgets/UpcomingProRataSection";

export interface ActiveDashboardProps {
  period: BudgetPeriod;
  user: User;
  /** When true, hides editing controls and Log Expense CTA. */
  readOnly?: boolean;
}

export function ActiveDashboard({ period, user, readOnly = false }: ActiveDashboardProps) {
  const [showSettings, setShowSettings] = useState(false);
  const [currentPeriod, setCurrentPeriod] = useState(period);

  const { data, loading, trendMonths, setTrendMonths, refresh } =
    useDashboardData(currentPeriod.year, currentPeriod.month);

  function handlePeriodUpdated(updatedPeriod: BudgetPeriod) {
    setCurrentPeriod(updatedPeriod);
    setShowSettings(false);
    refresh();
  }

  const totalSpent = data.summary?.totalSpent ?? 0;
  const remaining = data.summary?.remaining ?? currentPeriod.budgetAmount;

  const monthName = new Date(currentPeriod.year, currentPeriod.month - 1).toLocaleString("en-US", {
    month: "long",
    year: "numeric",
  });

  if (loading) {
    return <DashboardSkeleton />;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <LayoutDashboard className="size-6 text-primary" />
          <h1 className="text-2xl font-bold">
            {readOnly ? monthName : "Dashboard"}
          </h1>
        </div>
        <div className="flex items-center gap-2">
          {!readOnly && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowSettings(!showSettings)}
                aria-label="Budget Settings"
              >
                <Settings2 className="size-4" />
                <span className="hidden sm:inline ml-1">Budget Settings</span>
              </Button>
              {/* Desktop-only Log Expense button (FAB handles mobile) */}
              <Button asChild className="hidden md:inline-flex">
                <Link to="/expenses/new">
                  <PlusCircle className="size-4" />
                  Log Expense
                </Link>
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Budget Settings Editor */}
      {showSettings && !readOnly && (
        <SectionErrorBoundary sectionName="Budget Settings">
          <BudgetSettingsEditor
            period={currentPeriod}
            currency={user.currency}
            onSaved={handlePeriodUpdated}
            onCancel={() => setShowSettings(false)}
          />
        </SectionErrorBoundary>
      )}

      {/* Summary Bar */}
      <SectionErrorBoundary sectionName="Summary">
        <SummaryBar
          budgetAmount={currentPeriod.budgetAmount}
          totalSpent={totalSpent}
          remaining={remaining}
          daysLeft={
            data.summary
              ? data.summary.daysInPeriod - data.summary.daysElapsed
              : Math.max(
                  0,
                  new Date(currentPeriod.year, currentPeriod.month, 0).getDate() -
                    new Date().getDate(),
                )
          }
          currency={user.currency}
        />
      </SectionErrorBoundary>

      {/* Category Gauges */}
      <SectionErrorBoundary sectionName="Category Gauges">
        {data.summary && <CategoryGauges summary={data.summary} currency={user.currency} />}
      </SectionErrorBoundary>

      {/* Pacing Indicator */}
      <SectionErrorBoundary sectionName="Spending Pace">
        {data.summary && <PacingIndicator summary={data.summary} currency={user.currency} />}
      </SectionErrorBoundary>

      {/* Upcoming Pro-rata */}
      <SectionErrorBoundary sectionName="Upcoming Pro-rata">
        {data.upcomingProRata.length > 0 && (
          <UpcomingProRataSection schedules={data.upcomingProRata} currency={user.currency} />
        )}
      </SectionErrorBoundary>

      {/* Charts: hidden on mobile per US-DASH-09 */}
      <div className="hidden md:block space-y-6">
        {/* Historical Comparison Widget */}
        <SectionErrorBoundary sectionName="Historical Comparison">
          {data.comparison && (
            <HistoricalComparisonWidget
              comparison={data.comparison}
              currency={user.currency}
            />
          )}
        </SectionErrorBoundary>

        {/* Monthly Trends Section */}
        <SectionErrorBoundary sectionName="Monthly Trends">
          {data.trendData && data.trendData.length > 0 && (
            <MonthlyTrendsSection
              trendData={data.trendData}
              trendMonths={trendMonths}
              onToggle={setTrendMonths}
              currency={user.currency}
            />
          )}
        </SectionErrorBoundary>

        {/* Tag Spending Chart */}
        <SectionErrorBoundary sectionName="Tag Spending">
          {data.tagSpending.length > 0 && (
            <TagSpendingChart tagSpending={data.tagSpending} currency={user.currency} />
          )}
        </SectionErrorBoundary>

        {/* Cumulative Spend Chart */}
        <SectionErrorBoundary sectionName="Cumulative Spending">
          {data.cumulativeData.length > 0 && (
            <CumulativeSpendChart
              data={data.cumulativeData}
              currency={user.currency}
            />
          )}
        </SectionErrorBoundary>
      </div>

      {/* Recent Expenses or Empty State */}
      <SectionErrorBoundary sectionName="Recent Expenses">
        {data.recentExpenses.length === 0 && !readOnly ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <Wallet className="mb-4 size-12 text-muted-foreground/50" />
              <h2 className="mb-2 text-lg font-semibold">No expenses yet</h2>
              <p className="mb-6 max-w-sm text-sm text-muted-foreground">
                Start tracking your spending by logging your first expense for this
                month.
              </p>
              <Button asChild>
                <Link to="/expenses/new">
                  <PlusCircle className="size-4" />
                  Log your first expense
                </Link>
              </Button>
            </CardContent>
          </Card>
        ) : data.recentExpenses.length > 0 ? (
          <RecentExpenses expenses={data.recentExpenses} currency={user.currency} />
        ) : null}
      </SectionErrorBoundary>
    </div>
  );
}
