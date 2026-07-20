import { http, HttpResponse } from "msw";
import type { TrendResponse } from "@gofin/core";
import { computeMockTrends } from "../data";
import { simulateLatency } from "./latency";

export const trendsHandlers = [
  http.get<never, never, TrendResponse>("/api/finance/spending/trends", async ({ request }) => {
    await simulateLatency();
    const url = new URL(request.url);
    const months = Number(url.searchParams.get("months") ?? "6");
    return HttpResponse.json({ trends: computeMockTrends(months) });
  }),
];
