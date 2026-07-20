import { http, HttpResponse } from "msw";
import type {
  SummaryResponse,
  TagSpendingResponse,
  CumulativeSpendResponse,
  HistoricalComparisonResponse,
} from "@gofin/core";
import {
  mockSummary,
  mockComparison,
  computeTagSpending,
  computeCumulativeSpend,
} from "../data";
import { simulateLatency } from "./latency";

export const dashboardHandlers = [
  http.get<never, never, SummaryResponse>("/api/finance/summary", async () => {
    await simulateLatency();
    return HttpResponse.json({ summary: mockSummary });
  }),

  http.get<never, never, TagSpendingResponse>("/api/finance/spending/by-tag", async () => {
    await simulateLatency();
    return HttpResponse.json({ tagSpending: computeTagSpending() });
  }),

  http.get<never, never, CumulativeSpendResponse>(
    "/api/finance/spending/cumulative",
    async () => {
      await simulateLatency();
      return HttpResponse.json({ points: computeCumulativeSpend() });
    },
  ),

  http.get<never, never, HistoricalComparisonResponse>(
    "/api/finance/spending/comparison",
    async () => {
      await simulateLatency();
      return HttpResponse.json({ comparison: mockComparison });
    },
  ),
];
