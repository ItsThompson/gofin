import { act, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";
import { buildPeriod, buildPeriodSummary, createMockApi } from "@gofin/test-utils";
import { vi } from "vitest";
import { renderWithRouter } from "@gofin/test-utils";
import { ActiveDashboard } from "../components/ActiveDashboard";
import { dashboardDataEmptyRoutes, testPeriod, testSummary } from "./fixtures";

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function emptyDashboardApi() {
  return createMockApi({
    "/api/finance/periods/current": { body: { period: testPeriod } },
    ...dashboardDataEmptyRoutes(),
    "/api/finance/health-score/trend": { body: { trends: [] } },
    "/api/finance/health-score": { body: { healthScore: { configureBudget: true } } },
  });
}

describe("ActiveDashboard request races", () => {
  it("ignores a late response from the previous period", async () => {
    const nextPeriod = buildPeriod({ ...testPeriod, month: 6, budgetAmount: 400000 });
    const nextSummary = buildPeriodSummary({ ...testSummary, periodId: nextPeriod.id, month: 6, totalBudget: 400000, remaining: 400000 });
    let summaryCalls = 0;
    let resolveOldSummary!: (value: Response) => void;
    const fallback = emptyDashboardApi();
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes("/api/finance/summary")) {
        summaryCalls += 1;
        if (summaryCalls === 1) return new Promise<Response>((resolve) => { resolveOldSummary = resolve; });
        return Promise.resolve(response({ summary: nextSummary }));
      }
      return fallback(input, init);
    }) as typeof fetch;

    const view = render(<MemoryRouter><ActiveDashboard period={testPeriod} /></MemoryRouter>);
    await waitFor(() => expect(summaryCalls).toBe(1));
    view.rerender(<MemoryRouter><ActiveDashboard period={nextPeriod} /></MemoryRouter>);
    await waitFor(() => expect(summaryCalls).toBe(2));
    await waitFor(() => expect(screen.getAllByText("$4,000.00").length).toBeGreaterThan(0));

    await act(async () => resolveOldSummary(response({ summary: testSummary })));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText("$3,000.00")).not.toBeInTheDocument();
  });

  it("ignores superseded Summary Retry verification", async () => {
    let periodCalls = 0;
    const periodResolvers: Array<(value: Response) => void> = [];
    let summaryCalls = 0;
    const fallback = emptyDashboardApi();
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/finance/periods/current")) {
        periodCalls += 1;
        return new Promise<Response>((resolve) => { periodResolvers.push(resolve); });
      }
      if (url.includes("/api/finance/summary")) {
        summaryCalls += 1;
        return summaryCalls === 1
          ? Promise.resolve(response({ code: "UNAVAILABLE", message: "Summary failed" }, 503))
          : Promise.resolve(response({ summary: testSummary }));
      }
      return fallback(input, init);
    }) as typeof fetch;

    const user = userEvent.setup();
    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await screen.findByText("Could not load Summary");
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(periodCalls).toBe(1));
    await user.click(screen.getByRole("button", { name: "Refresh all data" }));
    await waitFor(() => expect(periodCalls).toBe(2));

    await act(async () => periodResolvers[0]?.(response({ period: testPeriod })));
    expect(summaryCalls).toBe(1);
    await act(async () => periodResolvers[1]?.(response({ period: testPeriod })));
    await waitFor(() => expect(summaryCalls).toBe(2));
    expect(screen.queryByText("Could not load the dashboard")).not.toBeInTheDocument();
  });

  it("does not retry an unrelated failed section during Summary Retry", async () => {
    let summaryCalls = 0;
    let byTagCalls = 0;
    const fallback = emptyDashboardApi();
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/finance/summary")) {
        summaryCalls += 1;
        return Promise.resolve(summaryCalls === 1
          ? response({ code: "UNAVAILABLE", message: "Summary failed" }, 503)
          : response({ summary: testSummary }));
      }
      if (url.includes("/api/finance/spending/by-tag")) {
        byTagCalls += 1;
        return Promise.resolve(response({ code: "UNAVAILABLE", message: "Tag spending failed" }, 503));
      }
      return fallback(input, init);
    }) as typeof fetch;

    const user = userEvent.setup();
    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    const summaryAlert = await screen.findByText("Could not load Summary");
    await screen.findByText("Could not load Tag spending");
    expect(byTagCalls).toBe(1);

    await user.click(within(summaryAlert.closest('[role="alert"]') as HTMLElement).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(summaryCalls).toBe(2));
    expect(byTagCalls).toBe(1);
    expect(screen.getByText("Could not load Tag spending")).toBeInTheDocument();
  });

  it("stops after one automatic summary period-miss retry", async () => {
    let summaryCalls = 0;
    let periodCalls = 0;
    const fallback = emptyDashboardApi();
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/finance/summary")) {
        summaryCalls += 1;
        return Promise.resolve(response({ code: "PERIOD_NOT_FOUND", message: "Missing" }, 404));
      }
      if (url.includes("/api/finance/periods/current")) {
        periodCalls += 1;
        return Promise.resolve(response({ period: testPeriod }));
      }
      return fallback(input, init);
    }) as typeof fetch;

    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await waitFor(() => expect(summaryCalls).toBe(2));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(summaryCalls).toBe(2);
    expect(periodCalls).toBe(1);
    expect(screen.getByText("Could not load Summary")).toBeInTheDocument();
  });

  it("pages suggestions and ignores a stale selection response", async () => {
    const firstSuggestionResolvers: Array<(value: Response) => void> = [];
    let suggestionCalls = 0;
    const fallback = emptyDashboardApi();
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/expenses/suggestions")) {
        suggestionCalls += 1;
        if (suggestionCalls === 1) return new Promise<Response>((resolve) => { firstSuggestionResolvers.push(resolve); });
        if (url.includes("page=1")) return Promise.resolve(response({ data: [{ name: "Old", tagId: "tag-old", originalTransactionAmountInMinorUnits: 100, transactionCurrencyCode: "USD", expenseType: "desires", frequency: 1, lastUsedAt: "2026-05-01", recencyBucket: "older", frecencyScore: 1 }], total: 2, page: 1, pageSize: 10, hasMore: true }));
        return Promise.resolve(response({ data: [{ name: "Current", tagId: "tag-current", originalTransactionAmountInMinorUnits: 100, transactionCurrencyCode: "USD", expenseType: "desires", frequency: 2, lastUsedAt: "2026-05-02", recencyBucket: "today", frecencyScore: 2 }], total: 2, page: 2, pageSize: 10, hasMore: false }));
      }
      return fallback(input, init);
    }) as typeof fetch;

    const user = userEvent.setup();
    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await waitFor(() => expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument());
    await user.click(screen.getByLabelText("Select breakdown chart"));
    await user.click(await screen.findByRole("option", { name: "Repeated Expenses" }));
    await waitFor(() => expect(suggestionCalls).toBe(1));
    await user.click(screen.getByLabelText("Select breakdown chart"));
    await user.click(await screen.findByRole("option", { name: "Spending by Tag" }));
    await act(async () => firstSuggestionResolvers[0]?.(response({ data: [{ name: "Stale", tagId: "tag-stale", originalTransactionAmountInMinorUnits: 100, transactionCurrencyCode: "USD", expenseType: "desires", frequency: 1, lastUsedAt: "2026-05-02", recencyBucket: "today", frecencyScore: 1 }], total: 1, page: 1, pageSize: 10, hasMore: false })));
    await user.click(screen.getByLabelText("Select breakdown chart"));
    await user.click(await screen.findByRole("option", { name: "Repeated Expenses" }));

    await waitFor(() => expect(screen.getByLabelText("Repeated expense details")).toHaveTextContent("Current"));
    expect(screen.getByLabelText("Repeated expense details")).not.toHaveTextContent("Stale");
    expect(suggestionCalls).toBe(3);
  });

  it("reloads a failed desktop section after returning from mobile", async () => {
    let viewportListener: ((event: MediaQueryListEvent) => void) | undefined;
    let byTagCalls = 0;
    const fallback = emptyDashboardApi();
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: true,
      addEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => { viewportListener = listener; },
      removeEventListener: vi.fn(),
    }));
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes("/api/finance/spending/by-tag")) {
        byTagCalls += 1;
        return Promise.resolve(byTagCalls === 1
          ? response({ code: "UNAVAILABLE", message: "Tag spending failed" }, 503)
          : response({ tagSpending: [] }));
      }
      return fallback(input, init);
    }) as typeof fetch;

    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await screen.findByText("Could not load Tag spending");
    act(() => viewportListener?.({ matches: false } as MediaQueryListEvent));
    act(() => viewportListener?.({ matches: true } as MediaQueryListEvent));
    await waitFor(() => expect(byTagCalls).toBe(2));
    expect(await screen.findByText("No tag spending is available.")).toBeInTheDocument();
  });

  it("aborts hidden desktop requests and reloads them when the viewport returns", async () => {
    let viewportListener: ((event: MediaQueryListEvent) => void) | undefined;
    let byTagCalls = 0;
    let resolveHiddenRequest!: (value: Response) => void;
    const fallback = emptyDashboardApi();
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: false,
      addEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => { viewportListener = listener; },
      removeEventListener: vi.fn(),
    }));
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes("/api/finance/spending/by-tag")) {
        byTagCalls += 1;
        if (byTagCalls === 1) return new Promise<Response>((resolve) => { resolveHiddenRequest = resolve; });
        return Promise.resolve(response({ tagSpending: [] }));
      }
      return fallback(input, init);
    }) as typeof fetch;

    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await waitFor(() => expect(screen.getByText("No expenses yet")).toBeInTheDocument());
    act(() => viewportListener?.({ matches: true } as MediaQueryListEvent));
    await waitFor(() => expect(byTagCalls).toBe(1));
    act(() => viewportListener?.({ matches: false } as MediaQueryListEvent));
    await act(async () => resolveHiddenRequest(response({ tagSpending: [{ tagName: "Stale" }] })));
    act(() => viewportListener?.({ matches: true } as MediaQueryListEvent));
    await waitFor(() => expect(byTagCalls).toBe(2));
    expect(screen.queryByText("Stale")).not.toBeInTheDocument();
  });
});
