import { http, HttpResponse } from "msw";
import type { HealthScoreResponse, HealthScoreTrendResponse } from "@gofin/core";
import { mockHealthScore, computeMockHealthScoreTrend } from "../data";
import { simulateLatency } from "./latency";

export const healthScoreHandlers = [
  http.get<never, never, HealthScoreResponse>("/api/finance/health-score", async () => {
    await simulateLatency();
    return HttpResponse.json({ healthScore: mockHealthScore });
  }),

  http.get<never, never, HealthScoreTrendResponse>(
    "/api/finance/health-score/trend",
    async ({ request }) => {
      await simulateLatency();
      const url = new URL(request.url);
      const months = Number(url.searchParams.get("months") ?? "6");
      return HttpResponse.json({ trends: computeMockHealthScoreTrend(months) });
    },
  ),
];
