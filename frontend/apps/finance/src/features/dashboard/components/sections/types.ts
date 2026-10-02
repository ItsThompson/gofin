import type { BudgetPeriod } from "@gofin/core";
import type { BreakdownChart } from "../../types";

export interface DashboardSectionProps {
  period: BudgetPeriod;
  enabled: boolean;
  refreshVersion: number;
  currency: string;
}

export interface DashboardDesktopSectionProps extends DashboardSectionProps {
  desktopVisible: boolean;
}

export interface DesktopDashboardProps extends DashboardSectionProps {
  initialTrendMonths: 6 | 12;
  initialSelectedChart: BreakdownChart;
  onTrendMonthsChange: (months: 6 | 12) => void;
  onSelectedChartChange: (chart: BreakdownChart) => void;
}

export interface DashboardSummarySectionProps extends DashboardSectionProps {
  summaryRetryVersion: number;
  onPeriodNotFound: () => boolean;
  onSuccess: () => void;
  onRetry: () => void;
}
