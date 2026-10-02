import { useState } from "react";
import { useDashboardViewport } from "../../hooks/useDashboardViewport";
import type { BreakdownChart } from "../../types";
import { BreakdownDashboardSection } from "./BreakdownDashboardSection";
import { CumulativeSpendingDashboardSection } from "./CumulativeSpendingDashboardSection";
import { HistoricalComparisonDashboardSection } from "./HistoricalComparisonDashboardSection";
import { TrendsDashboardSection } from "./TrendsDashboardSection";
import type { DesktopDashboardProps } from "./types";

export function DesktopDashboard({ period, enabled, refreshVersion, currency, initialTrendMonths, initialSelectedChart, onTrendMonthsChange, onSelectedChartChange }: DesktopDashboardProps) {
  const desktopVisible = useDashboardViewport();
  const [trendMonths, setTrendMonths] = useState<6 | 12>(initialTrendMonths);
  const [selectedChart, setSelectedChart] = useState<BreakdownChart>(initialSelectedChart);
  const sectionProps = { period, enabled, refreshVersion, currency, desktopVisible };

  return (
    <div className="hidden md:block space-y-6">
      {desktopVisible && <>
        <HistoricalComparisonDashboardSection {...sectionProps} />
        <TrendsDashboardSection {...sectionProps} trendMonths={trendMonths} onTrendMonthsChange={(months) => { setTrendMonths(months); onTrendMonthsChange(months); }} />
        <BreakdownDashboardSection {...sectionProps} selectedChart={selectedChart} onSelectedChartChange={(chart) => { setSelectedChart(chart); onSelectedChartChange(chart); }} />
        <CumulativeSpendingDashboardSection {...sectionProps} />
      </>}
    </div>
  );
}
