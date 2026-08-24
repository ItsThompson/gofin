import { http, HttpResponse } from "msw";
import type {
  Expense,
  CreateProRataRequest,
  ProRataResponse,
  UpcomingProRataResponse,
} from "@gofin/core";
import { currentMockUser, mockUpcomingProRata } from "../data";
import { simulateLatency } from "./latency";

export const prorataHandlers = [
  http.post<never, CreateProRataRequest, ProRataResponse>(
    "/api/finance/prorata",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const expense: Expense = {
        id: crypto.randomUUID(),
        userId: currentMockUser.id,
        name: body.name,
        transactionCurrency: body.transactionCurrency,
        transactionAmount: Math.round(body.totalAmount / body.months),
        reportingAmount: Math.round(body.totalAmount / body.months),
        reportingCurrency: body.transactionCurrency,
        expenseType: body.expenseType,
        tagId: body.tagId,
        expenseDate: body.expenseDate,
        periodYear: new Date().getFullYear(),
        periodMonth: new Date().getMonth() + 1,
        status: "active",
        isProRata: true,
        proRataGroup: crypto.randomUUID(),
        proRataIndex: 1,
        proRataTotal: body.months,
        createdAt: new Date().toISOString(),
      };
      return HttpResponse.json({ expense, schedules: [] }, { status: 201 });
    },
  ),

  http.get<never, never, UpcomingProRataResponse>(
    "/api/finance/prorata/upcoming",
    async () => {
      await simulateLatency();
      return HttpResponse.json({ schedules: mockUpcomingProRata });
    },
  ),
];
