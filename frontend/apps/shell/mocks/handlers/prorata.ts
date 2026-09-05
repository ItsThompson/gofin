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
        transactionCurrencyCode: body.transactionCurrencyCode,
        originalTransactionAmountInMinorUnits: Math.round(body.totalAmountInMinorUnits / body.spreadOverMonths),
        reportingAmountInMinorUnits: Math.round(body.totalAmountInMinorUnits / body.spreadOverMonths),
        reportingCurrencyCode: body.transactionCurrencyCode,
        expenseType: body.expenseType,
        tagId: body.tagId,
        expenseDateIso: body.expenseDateIso,
        periodYear: new Date().getFullYear(),
        periodMonth: new Date().getMonth() + 1,
        status: "active",
        isProRata: true,
        proRataGroup: crypto.randomUUID(),
        proRataIndex: 1,
        proRataTotal: body.spreadOverMonths,
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
