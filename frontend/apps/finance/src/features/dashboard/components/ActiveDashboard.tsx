import { useRef, useState } from "react";
import { Link } from "react-router";
import type { BudgetPeriod } from "@gofin/core";
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
import { useExpenseFrecencyData } from "../hooks/useExpenseFrecencyData";
import { BudgetSettingsEditor } from "./BudgetSettingsEditor";
import { TrendsSection } from "./TrendsSection";
import { BreakdownSection } from "./BreakdownSection";
import { DashboardOutline } from "./DashboardOutline";
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
  /** When true, hides editing controls and Log Expense CTA. */
  readOnly?: boolean;
}

export function ActiveDashboard({ period, readOnly = false }: ActiveDashboardProps) {
  const [showSettings, setShowSettings] = useState(false);
  const [currentPeriod, setCurrentPeriod] = useState(period);
  const dashboardContentRef = useRef<HTMLDivElement | null>(null);

  const { data, loading, trendMonths, setTrendMonths, refresh } =
    useDashboardData(currentPeriod.year, currentPeriod.month);
  const expenseFrecencyData = useExpenseFrecencyData({ pageSize: 10 });

  function handlePeriodUpdated(updatedPeriod: BudgetPeriod) {
    setCurrentPeriod(updatedPeriod);
    setShowSettings(false);
    refresh();
  }

  const totalSpent = data.summary?.totalSpent ?? 0;
  const remaining = data.summary?.remaining ?? currentPeriod.budgetAmount;
  const reportingCurrency = currentPeriod.reportingCurrency;

  const monthName = new Date(currentPeriod.year, currentPeriod.month - 1).toLocaleString("en-US", {
    month: "long",
    year: "numeric",
  });

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

      {showSettings && !readOnly && (
        <SectionErrorBoundary sectionName="Budget Settings">
          <BudgetSettingsEditor
            period={currentPeriod}
            onSaved={handlePeriodUpdated}
            onCancel={() => setShowSettings(false)}
          />
        </SectionErrorBoundary>
      )}

      {loading ? (
        <DashboardSkeleton />
      ) : (
        <>
          <div ref={dashboardContentRef} className="space-y-6">
            {/* Financial Health Score: first section, visible on mobile. */}
            {data.healthScore && (
              <section id="health-score" data-outline-title="Health Score">
                <SectionErrorBoundary sectionName="Health Score">
                  <HealthScoreCard score={data.healthScore} trend={data.healthScoreTrend} />
                </SectionErrorBoundary>
              </section>
            )}

            <section id="summary" data-outline-title="Summary" className="space-y-6">
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
                  currency={reportingCurrency}
                />
              </SectionErrorBoundary>

              {data.summary && (
                <section id="budget-allocations" data-outline-title="Budget Allocations">
                  <SectionErrorBoundary sectionName="Category Gauges">
                    <CategoryGauges summary={data.summary} currency={reportingCurrency} />
                  </SectionErrorBoundary>
                </section>
              )}

              {/* Spending Pace + Historical Comparison: side-by-side on desktop */}
              {(data.summary || data.comparison) && (
                <div className="hidden md:grid md:grid-cols-2 md:gap-6">
                  {data.summary && (
                    <section id="spending-pace" data-outline-title="Spending Pace">
                      <SectionErrorBoundary sectionName="Spending Pace">
                        <PacingIndicator summary={data.summary} currency={reportingCurrency} />
                      </SectionErrorBoundary>
                    </section>
                  )}
                  {data.comparison && (
                    <section id="historical-comparison" data-outline-title="Historical Comparison">
                      <SectionErrorBoundary sectionName="Historical Comparison">
                        <HistoricalComparisonWidget
                          comparison={data.comparison}
                          currency={reportingCurrency}
                        />
                      </SectionErrorBoundary>
                    </section>
                  )}
                </div>
              )}
            </section>

            <SectionErrorBoundary sectionName="Upcoming Pro-rata">
              {data.upcomingProRata.length > 0 && (
                <section id="upcoming-prorata" data-outline-title="Upcoming Pro-rata">
                  <UpcomingProRataSection
                    schedules={data.upcomingProRata}
                    currency={reportingCurrency}
                  />
                </section>
              )}
            </SectionErrorBoundary>

            {/* Charts: hidden on mobile */}
            <div className="hidden md:block space-y-6">
              {data.trendData && data.trendData.length > 0 && (
                <section id="trends" data-outline-title="Trends">
                  <SectionErrorBoundary sectionName="Monthly Trends">
                    <TrendsSection
                      trendData={data.trendData}
                      trendMonths={trendMonths}
                      onToggle={setTrendMonths}
                      currency={reportingCurrency}
                    />
                  </SectionErrorBoundary>
                </section>
              )}

              <section id="breakdown" data-outline-title="Breakdown">
                <SectionErrorBoundary sectionName="Breakdown">
                  <BreakdownSection
                    tagSpending={data.tagSpending}
                    expenseFrecencyData={expenseFrecencyData}
                    currency={reportingCurrency}
                  />
                </SectionErrorBoundary>
              </section>

              {data.cumulativeData.length > 0 && (
                <section id="cumulative-spending" data-outline-title="Cumulative Spending">
                  <SectionErrorBoundary sectionName="Cumulative Spending">
                    <CumulativeSpendChart
                      data={data.cumulativeData}
                      currency={reportingCurrency}
                    />
                  </SectionErrorBoundary>
                </section>
              )}
            </div>

            {/* Recent Expenses or Empty State */}
            {(data.recentExpenses.length > 0 || !readOnly) && (
              <section id="recent-expenses" data-outline-title="Recent Expenses">
                <SectionErrorBoundary sectionName="Recent Expenses">
                  {data.recentExpenses.length === 0 ? (
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
                  ) : (
                    <RecentExpenses expenses={data.recentExpenses} currency={reportingCurrency} />
                  )}
                </SectionErrorBoundary>
              </section>
            )}
          </div>

          {/* Dashboard Outline (TOC) - fixed positioned, renders on 2xl+ viewports */}
          <DashboardOutline rootRef={dashboardContentRef} />
        </>
      )}
    </div>
  );
}
