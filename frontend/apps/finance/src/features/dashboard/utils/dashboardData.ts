import { ApiRequestError } from "@gofin/api";
import type { BudgetPeriod } from "@gofin/core";
import type {
  DashboardData,
  DashboardSectionState,
  DashboardControllerStatus,
  SectionState,
} from "../types";

export function getPeriodErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) return error.message;
  if (error instanceof Error) return error.message;
  return "This period is unavailable right now.";
}

export function getSectionData<T>(state: SectionState<T>): T | null {
  return state.status === "success" ? state.data : null;
}

export function isSamePeriod(first: BudgetPeriod, second: BudgetPeriod): boolean {
  return first.id === second.id
    && first.userId === second.userId
    && first.year === second.year
    && first.month === second.month
    && first.budgetAmount === second.budgetAmount
    && first.reportingCurrencyCode === second.reportingCurrencyCode
    && first.essentialsPercent === second.essentialsPercent
    && first.desiresPercent === second.desiresPercent
    && first.savingsPercent === second.savingsPercent
    && first.createdAt === second.createdAt
    && first.updatedAt === second.updatedAt;
}

export function buildDashboardData(sections: DashboardSectionState): DashboardData {
  const trendData = getSectionData(sections.trends);
  const healthScoreTrend = getSectionData(sections.healthScoreTrend);

  return {
    summary: getSectionData(sections.summary),
    tagSpending: [...(getSectionData(sections.byTag) ?? [])],
    cumulativeData: [...(getSectionData(sections.cumulative) ?? [])],
    recentExpenses: [...(getSectionData(sections.recentExpenses) ?? [])],
    comparison: getSectionData(sections.comparison),
    upcomingProRata: [...(getSectionData(sections.upcomingProRata) ?? [])],
    trendData: trendData ? [...trendData] : null,
    healthScore: getSectionData(sections.healthScore),
    healthScoreTrend: healthScoreTrend ? [...healthScoreTrend] : null,
  };
}

export function isDashboardLoading(
  periodStatus: DashboardControllerStatus,
  sections: DashboardSectionState,
): boolean {
  return periodStatus !== "active"
    || Object.values(sections).some((section) => section.status === "loading");
}
