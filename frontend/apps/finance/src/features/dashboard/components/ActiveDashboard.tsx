import { useRef, useState } from "react";
import { Link } from "react-router";
import type { BudgetPeriod, User } from "@gofin/core";
import type { BreakdownChart } from "../types";
import { Button } from "@gofin/ui/components/button";
import { Card, CardContent, CardHeader, CardTitle } from "@gofin/ui/components/card";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";
import { LayoutDashboard, PlusCircle, Settings2, RefreshCw } from "lucide-react";
import { useDashboardPeriod } from "../hooks/useDashboardPeriod";
import { BudgetSettingsEditor } from "./BudgetSettingsEditor";
import { CreatePeriodPrompt } from "./CreatePeriodPrompt";
import { DashboardOutline } from "./DashboardOutline";
import {
  DesktopDashboard,
  HealthScoreDashboardSection,
  RecentExpensesDashboardSection,
  SummaryDashboardSection,
  UpcomingProRataDashboardSection,
} from "./sections";

export interface ActiveDashboardProps {
  period: BudgetPeriod;
  user?: User;
  readOnly?: boolean;
}

export function ActiveDashboard({ period, user, readOnly = false }: ActiveDashboardProps) {
  const [showSettings, setShowSettings] = useState(false);
  const dashboardContentRef = useRef<HTMLDivElement | null>(null);
  const trendMonthsRef = useRef<6 | 12>(6);
  const selectedChartRef = useRef<BreakdownChart>("tag-spending");
  const periodController = useDashboardPeriod(period, readOnly);
  const renderedPeriod = periodController.period;
  const dataEnabled = periodController.status === "active" && !periodController.verifying;
  const currency = renderedPeriod.reportingCurrencyCode;
  const monthName = new Date(renderedPeriod.year, renderedPeriod.month - 1).toLocaleString("en-US", {
    month: "long",
    year: "numeric",
  });

  if (periodController.status === "loading") return <DashboardSkeleton />;
  if (periodController.status === "no-period") {
    if (!user || !periodController.recovery) {
      return <Card role="alert"><CardContent>Could not restore this budget period.</CardContent></Card>;
    }
    return (
      <CreatePeriodPrompt
        defaults={periodController.recovery.defaults}
        user={user}
        year={period.year}
        month={period.month}
        onCreatePeriod={periodController.recovery.createPeriod}
        creating={periodController.recovery.creating}
        createError={periodController.recovery.createError}
      />
    );
  }
  if (periodController.status === "not-found") {
    return (
      <Card role="alert">
        <CardHeader><CardTitle className="text-destructive">Historical period not found</CardTitle></CardHeader>
        <CardContent><p className="text-sm text-muted-foreground">{periodController.error}</p></CardContent>
      </Card>
    );
  }
  if (periodController.status === "error") {
    return (
      <Card role="alert">
        <CardHeader><CardTitle className="text-destructive">Could not load the dashboard</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">{periodController.error}</p>
          <Button variant="outline" onClick={periodController.refresh}>Retry</Button>
        </CardContent>
      </Card>
    );
  }

  function handlePeriodUpdated(updatedPeriod: BudgetPeriod) {
    setShowSettings(false);
    periodController.replacePeriodAfterEdit(updatedPeriod);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <LayoutDashboard className="size-6 text-primary" />
          <h1 className="text-2xl font-bold">{readOnly ? monthName : "Dashboard"}</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={periodController.refresh} aria-label="Refresh all data">
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
        <HealthScoreDashboardSection period={renderedPeriod} enabled={dataEnabled} refreshVersion={periodController.refreshVersion} currency={currency} />
        <SummaryDashboardSection period={renderedPeriod} enabled={dataEnabled} refreshVersion={periodController.refreshVersion} summaryRetryVersion={periodController.summaryRetryVersion} currency={currency} onPeriodNotFound={periodController.onSummaryPeriodNotFound} onSuccess={periodController.onSummarySuccess} onRetry={periodController.retrySummary} />
        <UpcomingProRataDashboardSection period={renderedPeriod} enabled={dataEnabled} refreshVersion={periodController.refreshVersion} currency={currency} />
        <DesktopDashboard period={renderedPeriod} enabled={dataEnabled} refreshVersion={periodController.refreshVersion} currency={currency} initialTrendMonths={trendMonthsRef.current} initialSelectedChart={selectedChartRef.current} onTrendMonthsChange={(months) => { trendMonthsRef.current = months; }} onSelectedChartChange={(chart) => { selectedChartRef.current = chart; }} />
        <RecentExpensesDashboardSection period={renderedPeriod} enabled={dataEnabled} refreshVersion={periodController.refreshVersion} currency={currency} readOnly={readOnly} />
      </div>
      <DashboardOutline rootRef={dashboardContentRef} />
    </div>
  );
}
