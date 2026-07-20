import { http, HttpResponse } from "msw";
import type {
  BudgetPeriod,
  PeriodResponse,
  PeriodListResponse,
  CreatePeriodRequest,
  CreatePeriodResponse,
  UpdatePeriodRequest,
  ApiError,
} from "@gofin/core";
import { mockPeriod } from "../data";
import { simulateLatency } from "./latency";

export const periodsHandlers = [
  http.get<never, never, PeriodResponse | ApiError>(
    "/api/finance/periods/current",
    async ({ request }) => {
      await simulateLatency();
      const url = new URL(request.url);
      const year = Number(url.searchParams.get("year"));
      const month = Number(url.searchParams.get("month"));

      if (year === mockPeriod.year && month === mockPeriod.month) {
        return HttpResponse.json({ period: mockPeriod });
      }
      return HttpResponse.json(
        { code: "PERIOD_NOT_FOUND", message: "No budget period for this month" },
        { status: 404 },
      );
    },
  ),

  http.post<never, CreatePeriodRequest, CreatePeriodResponse>(
    "/api/finance/periods",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const period: BudgetPeriod = {
        ...mockPeriod,
        id: crypto.randomUUID(),
        ...body,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      return HttpResponse.json({ period, appliedProRata: [] }, { status: 201 });
    },
  ),

  http.put<{ id: string }, UpdatePeriodRequest, PeriodResponse>(
    "/api/finance/periods/:id",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const period: BudgetPeriod = { ...mockPeriod, ...body, updatedAt: new Date().toISOString() };
      return HttpResponse.json({ period });
    },
  ),

  http.get<never, never, PeriodListResponse>("/api/finance/periods", async () => {
    await simulateLatency();
    return HttpResponse.json({ periods: [mockPeriod] });
  }),
];
