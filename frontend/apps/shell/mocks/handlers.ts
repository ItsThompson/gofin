import { http, HttpResponse, delay } from "msw";
import {
  currentMockUser,
  mockPeriod,
  mockDefaults,
  mockExpenses,
  mockTags,
  mockSummary,
  mockComparison,
  mockHealthScore,
  computeMockHealthScoreTrend,
  mockUpcomingProRata,
  allUsers,
  computeTagSpending,
  computeCumulativeSpend,
  computeMockTrends,
} from "./data";

/** Simulates realistic network latency. */
async function simulateLatency(): Promise<void> {
  await delay(100 + Math.random() * 200);
}

export const handlers = [
  // ─── Auth ────────────────────────────────────────────────────────────

  http.get("/api/auth/me", async () => {
    await simulateLatency();
    return HttpResponse.json({ user: currentMockUser });
  }),

  http.post("/api/auth/login", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as { email: string; password: string };
    const user = allUsers.find((u) => u.email === body.email);
    if (!user) {
      return HttpResponse.json(
        { code: "INVALID_CREDENTIALS", message: "Invalid email or password" },
        { status: 401 },
      );
    }
    return HttpResponse.json({ user });
  }),

  http.post("/api/auth/register", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as { username: string; email: string };
    const newUser = {
      ...currentMockUser,
      id: crypto.randomUUID(),
      username: body.username,
      email: body.email,
      hasCompletedOnboarding: false,
    };
    return HttpResponse.json({ user: newUser });
  }),

  http.post("/api/auth/logout", async () => {
    await simulateLatency();
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("/api/auth/refresh", async () => {
    await simulateLatency();
    return new HttpResponse(null, { status: 204 });
  }),

  http.put("/api/auth/me", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, string>;
    const updated = { ...currentMockUser, ...body };
    return HttpResponse.json({ user: updated });
  }),

  http.post("/api/auth/me/password", async () => {
    await simulateLatency();
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("/api/auth/onboarding-complete", async () => {
    await simulateLatency();
    const updated = { ...currentMockUser, hasCompletedOnboarding: true };
    return HttpResponse.json({ user: updated });
  }),

  http.post("/api/auth/assume", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as { userId: string };
    const target = allUsers.find((u) => u.id === body.userId);
    if (!target) {
      return HttpResponse.json(
        { code: "NOT_FOUND", message: "User not found" },
        { status: 404 },
      );
    }
    return HttpResponse.json({ user: target });
  }),

  http.post("/api/auth/restore", async () => {
    await simulateLatency();
    return HttpResponse.json({ user: currentMockUser });
  }),

  // ─── Admin ───────────────────────────────────────────────────────────

  http.get("/api/admin/users", async () => {
    await simulateLatency();
    return HttpResponse.json({ users: allUsers });
  }),

  http.delete("/api/admin/users/:id", async ({ request, params }) => {
    await simulateLatency();
    const body = (await request.json()) as { password: string };
    const targetId = params.id as string;

    // Simulate password check: accept any non-empty password in dev-mock
    if (!body.password) {
      return HttpResponse.json(
        { code: "VALIDATION_ERROR", message: "Invalid request body" },
        { status: 400 },
      );
    }

    // Simulate wrong password with a known test value
    if (body.password === "wrong") {
      return HttpResponse.json(
        { code: "INVALID_CREDENTIALS", message: "Invalid password" },
        { status: 401 },
      );
    }

    const target = allUsers.find((u) => u.id === targetId);
    if (!target) {
      return HttpResponse.json(
        { code: "NOT_FOUND", message: "User not found" },
        { status: 404 },
      );
    }

    // Simulate protected user check
    if (target.username === "admin" || target.username === "thompson") {
      return HttpResponse.json(
        { code: "PROTECTED_USER", message: "Cannot delete a protected user" },
        { status: 403 },
      );
    }

    return new HttpResponse(null, { status: 204 });
  }),

  // ─── Finance: Periods ────────────────────────────────────────────────

  http.get("/api/finance/periods/current", async ({ request }) => {
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
  }),

  http.post("/api/finance/periods", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const newPeriod = {
      ...mockPeriod,
      id: crypto.randomUUID(),
      ...body,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    return HttpResponse.json({ period: newPeriod, appliedProRata: [] }, { status: 201 });
  }),

  http.put("/api/finance/periods/:id", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const updated = { ...mockPeriod, ...body, updatedAt: new Date().toISOString() };
    return HttpResponse.json({ period: updated });
  }),

  http.get("/api/finance/periods", async () => {
    await simulateLatency();
    return HttpResponse.json({
      data: [mockPeriod],
      total: 1,
      page: 1,
      pageSize: 20,
      hasMore: false,
    });
  }),

  // ─── Finance: Defaults ───────────────────────────────────────────────

  http.get("/api/finance/defaults", async () => {
    await simulateLatency();
    return HttpResponse.json({ defaults: mockDefaults });
  }),

  http.put("/api/finance/defaults", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const updated = { ...mockDefaults, ...body, updatedAt: new Date().toISOString() };
    return HttpResponse.json({ defaults: updated });
  }),

  http.post("/api/finance/onboarding", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const updated = { ...mockDefaults, ...body, updatedAt: new Date().toISOString() };
    return HttpResponse.json({ defaults: updated });
  }),

  // ─── Finance: Tags ───────────────────────────────────────────────────

  http.get("/api/finance/tags", async () => {
    await simulateLatency();
    return HttpResponse.json({ tags: mockTags });
  }),

  http.post("/api/finance/tags", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as { name: string };
    const newTag = {
      id: crypto.randomUUID(),
      name: body.name,
      isDefault: false,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    return HttpResponse.json({ tag: newTag }, { status: 201 });
  }),

  http.put("/api/finance/tags/:id", async ({ request, params }) => {
    await simulateLatency();
    const body = (await request.json()) as { name: string };
    const existing = mockTags.find((tag) => tag.id === params.id);
    const updated = {
      ...(existing ?? { id: params.id, isDefault: false, createdAt: new Date().toISOString() }),
      name: body.name,
      updatedAt: new Date().toISOString(),
    };
    return HttpResponse.json({ tag: updated });
  }),

  http.delete("/api/finance/tags/:id", async ({ params }) => {
    await simulateLatency();
    const tag = mockTags.find((t) => t.id === params.id);
    if (tag?.isDefault) {
      return HttpResponse.json(
        { code: "DEFAULT_TAG", message: "Default tags cannot be deleted" },
        { status: 400 },
      );
    }
    return new HttpResponse(null, { status: 204 });
  }),

  // ─── Finance: Pro-rata ───────────────────────────────────────────────

  http.post("/api/finance/prorata", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const expense = {
      id: crypto.randomUUID(),
      name: body.name,
      amount: Math.round((body.totalAmount as number) / (body.months as number)),
      currency: body.currency,
      expenseType: body.expenseType,
      tagId: body.tagId,
      expenseDate: body.expenseDate,
      periodYear: new Date().getFullYear(),
      periodMonth: new Date().getMonth() + 1,
      isProRata: true,
      proRataGroup: crypto.randomUUID(),
      proRataIndex: 1,
      proRataTotal: body.months,
      createdAt: new Date().toISOString(),
    };
    return HttpResponse.json({ expense, schedules: [] }, { status: 201 });
  }),

  http.get("/api/finance/prorata/upcoming", async () => {
    await simulateLatency();
    return HttpResponse.json({ schedules: mockUpcomingProRata });
  }),

  // ─── Finance: Dashboard Aggregations ─────────────────────────────────

  http.get("/api/finance/summary", async () => {
    await simulateLatency();
    return HttpResponse.json({ summary: mockSummary });
  }),

  http.get("/api/finance/spending/by-tag", async () => {
    await simulateLatency();
    return HttpResponse.json({ tagSpending: computeTagSpending() });
  }),

  http.get("/api/finance/spending/cumulative", async () => {
    await simulateLatency();
    return HttpResponse.json({ points: computeCumulativeSpend() });
  }),

  http.get("/api/finance/spending/comparison", async () => {
    await simulateLatency();
    return HttpResponse.json({ comparison: mockComparison });
  }),

  http.get("/api/finance/health-score", async () => {
    await simulateLatency();
    return HttpResponse.json({ healthScore: mockHealthScore });
  }),

  http.get("/api/finance/health-score/trend", async ({ request }) => {
    await simulateLatency();
    const url = new URL(request.url);
    const months = Number(url.searchParams.get("months") ?? "6");
    return HttpResponse.json({ trends: computeMockHealthScoreTrend(months) });
  }),

  http.get("/api/finance/spending/trends", async ({ request }) => {
    await simulateLatency();
    const url = new URL(request.url);
    const months = Number(url.searchParams.get("months") ?? "6");
    return HttpResponse.json({ trends: computeMockTrends(months) });
  }),

  // ─── Expenses ────────────────────────────────────────────────────────

  http.get("/api/expenses", async ({ request }) => {
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

  http.post("/api/expenses", async ({ request }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const expense = {
      id: crypto.randomUUID(),
      userId: currentMockUser.id,
      ...body,
      status: "active",
      isProRata: false,
      createdAt: new Date().toISOString(),
    };
    return HttpResponse.json({ expense }, { status: 201 });
  }),

  http.get("/api/expenses/:id", async ({ params }) => {
    await simulateLatency();
    const expense = mockExpenses.find((e) => e.id === params.id);
    if (!expense) {
      return HttpResponse.json(
        { code: "NOT_FOUND", message: "Expense not found" },
        { status: 404 },
      );
    }
    return HttpResponse.json({ expense });
  }),

  http.post("/api/expenses/:id/correct", async ({ request, params }) => {
    await simulateLatency();
    const body = (await request.json()) as Record<string, unknown>;
    const original = mockExpenses.find((e) => e.id === params.id);
    const correction = {
      ...(original ?? {}),
      id: crypto.randomUUID(),
      ...body,
      status: "active",
      correctsId: params.id,
      createdAt: new Date().toISOString(),
    };
    return HttpResponse.json({ expense: correction }, { status: 201 });
  }),

  http.get("/api/expenses/:id/history", async ({ params }) => {
    await simulateLatency();
    const expense = mockExpenses.find((e) => e.id === params.id);
    return HttpResponse.json({ entries: expense ? [expense] : [] });
  }),

  http.get("/api/expenses/prorata/:groupId", async () => {
    await simulateLatency();
    return HttpResponse.json({ expenses: [] });
  }),

  // ─── Health (for E2E stack check) ────────────────────────────────────

  http.get("/api/health", () => {
    return HttpResponse.json({ status: "ok" });
  }),
];
