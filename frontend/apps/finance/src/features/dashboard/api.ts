import { apiClient } from "@gofin/api";
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
} from "../../types";

export const dashboardApi = {
  getCurrentPeriod: (year: number, month: number) =>
    apiClient<PeriodResponse>(
      `/api/finance/periods/current?year=${year}&month=${month}`,
    ),

  getSummary: (year: number, month: number) =>
    apiClient<SummaryResponse>(
      `/api/finance/summary?year=${year}&month=${month}`,
    ),

  getTagSpending: (year: number, month: number) =>
    apiClient<TagSpendingResponse>(
      `/api/finance/spending/by-tag?year=${year}&month=${month}`,
    ),

  getCumulative: (year: number, month: number) =>
    apiClient<CumulativeSpendResponse>(
      `/api/finance/spending/cumulative?year=${year}&month=${month}`,
    ),

  getComparison: (year: number, month: number) =>
    apiClient<HistoricalComparisonResponse>(
      `/api/finance/spending/comparison?year=${year}&month=${month}`,
    ),

  getUpcomingProRata: () =>
    apiClient<UpcomingProRataResponse>(`/api/finance/prorata/upcoming`),

  getRecentExpenses: (year: number, month: number, pageSize: number) =>
    apiClient<PaginatedResponse<Expense>>(
      `/api/expenses?year=${year}&month=${month}&page=1&pageSize=${pageSize}`,
    ),

  getTrend: (year: number, month: number, months: number) =>
    apiClient<TrendResponse>(
      `/api/finance/spending/trends?year=${year}&month=${month}&months=${months}`,
    ),

  getDefaults: () =>
    apiClient<DefaultsResponse>("/api/finance/defaults"),

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
