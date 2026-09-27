import { apiClient } from "@gofin/api";
import type { ApiClientOptions } from "@gofin/api";
import type { PaginatedResponse } from "@gofin/core";
import type {
  PeriodResponse,
  SummaryResponse,
  TagSpendingResponse,
  CumulativeSpendResponse,
  HistoricalComparisonResponse,
  UpcomingProRataResponse,
  Expense,
  TrendResponse,
  DefaultsResponse,
  CreatePeriodRequest,
  CreatePeriodResponse,
  UpdatePeriodRequest,
  HealthScoreResponse,
  HealthScoreTrendResponse,
} from "@gofin/core";

export type DashboardRequestOptions = Pick<ApiClientOptions, "forceRefresh" | "signal">;

export const dashboardApi = {
  getCurrentPeriod: (year: number, month: number, options?: DashboardRequestOptions) =>
    apiClient<PeriodResponse>(
      `/api/finance/periods/current?year=${year}&month=${month}`,
      options,
    ),

  getSummary: (year: number, month: number, options?: DashboardRequestOptions) =>
    apiClient<SummaryResponse>(
      `/api/finance/summary?year=${year}&month=${month}`,
      options,
    ),

  getHealthScore: (year: number, month: number, options?: DashboardRequestOptions) =>
    apiClient<HealthScoreResponse>(
      `/api/finance/health-score?year=${year}&month=${month}`,
      options,
    ),

  getHealthScoreTrend: (year: number, month: number, months: number, options?: DashboardRequestOptions) =>
    apiClient<HealthScoreTrendResponse>(
      `/api/finance/health-score/trend?year=${year}&month=${month}&months=${months}`,
      options,
    ),

  getTagSpending: (year: number, month: number, options?: DashboardRequestOptions) =>
    apiClient<TagSpendingResponse>(
      `/api/finance/spending/by-tag?year=${year}&month=${month}`,
      options,
    ),

  getCumulative: (year: number, month: number, options?: DashboardRequestOptions) =>
    apiClient<CumulativeSpendResponse>(
      `/api/finance/spending/cumulative?year=${year}&month=${month}`,
      options,
    ),

  getComparison: (year: number, month: number, options?: DashboardRequestOptions) =>
    apiClient<HistoricalComparisonResponse>(
      `/api/finance/spending/comparison?year=${year}&month=${month}`,
      options,
    ),

  getUpcomingProRata: (options?: DashboardRequestOptions) =>
    apiClient<UpcomingProRataResponse>(`/api/finance/prorata/upcoming`, options),

  getRecentExpenses: (year: number, month: number, pageSize: number, options?: DashboardRequestOptions) =>
    apiClient<PaginatedResponse<Expense>>(
      `/api/expenses?year=${year}&month=${month}&page=1&pageSize=${pageSize}`,
      options,
    ),

  getTrend: (year: number, month: number, months: number, options?: DashboardRequestOptions) =>
    apiClient<TrendResponse>(
      `/api/finance/spending/trends?year=${year}&month=${month}&months=${months}`,
      options,
    ),

  getDefaults: (options?: DashboardRequestOptions) =>
    apiClient<DefaultsResponse>("/api/finance/defaults", options),

  createPeriod: (body: CreatePeriodRequest) =>
    apiClient<CreatePeriodResponse>("/api/finance/periods", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updatePeriod: (periodId: string, body: UpdatePeriodRequest) =>
    apiClient<PeriodResponse>(`/api/finance/periods/${periodId}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
};
