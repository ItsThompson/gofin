import { http, HttpResponse } from "msw";
import type {
  Expense,
  ExpenseResponse,
  CreateExpenseRequest,
  CorrectExpenseRequest,
  CorrectionHistoryResponse,
  PaginatedResponse,
  ApiError,
} from "@gofin/core";
import { mockExpenses, currentMockUser } from "../data";
import { simulateLatency } from "./latency";

export const expensesHandlers = [
  http.get<never, never, PaginatedResponse<Expense>>("/api/expenses", async ({ request }) => {
    await simulateLatency();
    const url = new URL(request.url);
    const page = Number(url.searchParams.get("page") ?? "1");
    const pageSize = Number(url.searchParams.get("pageSize") ?? "20");
    const start = (page - 1) * pageSize;
    const slice = mockExpenses.slice(start, start + pageSize);

    return HttpResponse.json({
      data: slice,
      total: mockExpenses.length,
      page,
      pageSize,
      hasMore: start + pageSize < mockExpenses.length,
    });
  }),

  http.post<never, CreateExpenseRequest, ExpenseResponse>(
    "/api/expenses",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const expense: Expense = {
        id: crypto.randomUUID(),
        userId: currentMockUser.id,
        ...body,
        transactionAmount: body.amount,
        reportingAmount: body.amount,
        reportingCurrency: body.transactionCurrency,
        status: "active",
        isProRata: false,
        createdAt: new Date().toISOString(),
      };
      return HttpResponse.json({ expense }, { status: 201 });
    },
  ),

  http.get<{ id: string }, never, ExpenseResponse | ApiError>(
    "/api/expenses/:id",
    async ({ params }) => {
      await simulateLatency();
      const expense = mockExpenses.find((e) => e.id === params.id);
      if (!expense) {
        return HttpResponse.json(
          { code: "NOT_FOUND", message: "Expense not found" },
          { status: 404 },
        );
      }
      return HttpResponse.json({ expense });
    },
  ),

  http.post<{ id: string }, CorrectExpenseRequest, ExpenseResponse>(
    "/api/expenses/:id/correct",
    async ({ request, params }) => {
      await simulateLatency();
      const body = await request.json();
      const original = mockExpenses.find((e) => e.id === params.id);
      const base: Expense = original ?? {
        id: params.id,
        userId: currentMockUser.id,
        name: body.name,
        amount: body.amount,
        transactionCurrency: currentMockUser.currency,
        transactionAmount: body.amount,
        reportingAmount: body.amount,
        reportingCurrency: currentMockUser.currency,
        expenseType: body.expenseType,
        tagId: body.tagId,
        expenseDate: body.expenseDate,
        periodYear: new Date().getFullYear(),
        periodMonth: new Date().getMonth() + 1,
        status: "active",
        isProRata: false,
        createdAt: new Date().toISOString(),
      };
      const correction: Expense = {
        ...base,
        id: crypto.randomUUID(),
        ...body,
        status: "active",
        correctsId: params.id,
        createdAt: new Date().toISOString(),
      };
      return HttpResponse.json({ expense: correction }, { status: 201 });
    },
  ),

  http.get<{ id: string }, never, CorrectionHistoryResponse>(
    "/api/expenses/:id/history",
    async ({ params }) => {
      await simulateLatency();
      const expense = mockExpenses.find((e) => e.id === params.id);
      return HttpResponse.json({ entries: expense ? [expense] : [] });
    },
  ),

  http.get<{ groupId: string }, never, PaginatedResponse<Expense>>(
    "/api/expenses/prorata/:groupId",
    async () => {
      await simulateLatency();
      return HttpResponse.json({ data: [], total: 0, page: 1, pageSize: 100, hasMore: false });
    },
  ),
];
