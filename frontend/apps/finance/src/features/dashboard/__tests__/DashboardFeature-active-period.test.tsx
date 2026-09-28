import { describe, it, expect } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { buildUser, buildPeriod, buildPeriodSummary, createMockApi, mockSequence } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import {
  testUser,
  testPeriod,
  testSummary,
  dashboardDataEmptyRoutes,
  dashboardDataWithExpensesRoutes,
} from "./fixtures";

describe("DashboardFeature", () => {
  it("renders skeleton loading state initially", () => {
    // Mock fetch that never resolves (simulates loading)
    globalThis.fetch = (() => new Promise(() => {})) as unknown as typeof fetch;
    renderDashboard();

    const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
    expect(document.querySelector('[data-testid="health-score-skeleton"]')).not.toBeNull();
  });

  describe("active period exists", () => {
    it("hides dashboard controls while automatic Summary recovery verifies the period", async () => {
      let currentPeriodCalls = 0;
      let summaryCalls = 0;
      let resolveAutomaticVerification!: (response: Response) => void;
      let resolveRecoveredSummary!: (response: Response) => void;
      const baseApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/health-score/trend": { body: { trends: [] } },
        "/api/finance/health-score": { body: { healthScore: { configureBudget: true } } },
      });
      globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes("/api/finance/periods/current")) {
          currentPeriodCalls += 1;
          if (currentPeriodCalls === 3) {
            return new Promise<Response>((resolve) => { resolveAutomaticVerification = resolve; });
          }
        }
        if (url.includes("/api/finance/summary")) {
          summaryCalls += 1;
          if (summaryCalls === 1) return baseApi(input, init);
          if (summaryCalls === 2) {
            return new Response(JSON.stringify({ code: "PERIOD_NOT_FOUND", message: "Period is missing" }), {
              status: 404,
              headers: { "Content-Type": "application/json" },
            });
          }
          return new Promise<Response>((resolve) => { resolveRecoveredSummary = resolve; });
        }
        return baseApi(input, init);
      }) as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => expect(screen.getByText("No expenses yet")).toBeInTheDocument());
      await user.click(screen.getByRole("button", { name: "Refresh all data" }));
      await waitFor(() => expect(summaryCalls).toBe(2));
      await waitFor(() => expect(currentPeriodCalls).toBe(3));

      expect(screen.queryByRole("button", { name: "Refresh all data" })).not.toBeInTheDocument();
      expect(screen.queryByRole("heading", { name: "Dashboard" })).not.toBeInTheDocument();
      expect(document.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);

      await act(async () => resolveAutomaticVerification(new Response(JSON.stringify({ period: testPeriod }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })));
      await waitFor(() => expect(summaryCalls).toBe(3));
      expect(screen.getByRole("button", { name: "Refresh all data" })).toBeInTheDocument();

      await act(async () => resolveRecoveredSummary(new Response(JSON.stringify({ summary: testSummary }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })));
      await waitFor(() => expect(screen.getByRole("button", { name: "Refresh all data" })).toBeInTheDocument());
    });

    it("shows the Log Expense link while dashboard data is loading", async () => {
      globalThis.fetch = ((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes("/api/finance/periods/current")) {
          return Promise.resolve(
            new Response(JSON.stringify({ period: testPeriod }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return new Promise(() => {});
      }) as unknown as typeof fetch;
      renderDashboard();

      // The period fetch resolves on a microtask after renderDashboard()
      // returns, so the header assertions must wait for ActiveDashboard to
      // mount (same pattern as the other period-dependent tests below).
      await waitFor(() => {
        expect(screen.getByRole("link", { name: "Log Expense" })).toHaveAttribute(
          "href",
          "/expenses/new",
        );
      });
      // Data routes never resolve, so the content area is still loading.
      expect(document.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
    });

    it("keeps the trend window selector available when six months has no data", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/spending/trends": mockSequence([
          { body: { trends: [] } },
          { body: { trends: [{ year: 2026, month: 1, totalSpent: 100, budgetAmount: 300000, essentialsSpent: 50, desiresSpent: 50, savingsSpent: 0, essentialsPercent: 50, desiresPercent: 30, savingsPercent: 20 }] } },
        ]),
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => expect(screen.getByLabelText("12 months")).toBeInTheDocument());
      await user.click(screen.getByLabelText("12 months"));
      await waitFor(() => {
        expect(screen.getAllByText("Monthly Spending").length).toBeGreaterThanOrEqual(1);
        expect(screen.getByText("Jan '26: $1.00 spent")).toBeInTheDocument();
      });
      expect(mockApi._calls.some((call) => call.url.includes("/api/finance/spending/trends") && call.url.includes("months=12"))).toBe(true);
    });

    it("shows pending states for deferred trend and breakdown selections", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/summary": { status: 500, body: { code: "INTERNAL_SERVER_ERROR", message: "Summary failed" } },
        "/api/finance/spending/trends": { body: { trends: [{ year: 2026, month: 5, totalSpent: 100, budgetAmount: 300000, essentialsSpent: 50, desiresSpent: 50, savingsSpent: 0, essentialsPercent: 50, desiresPercent: 30, savingsPercent: 20 }] } },
      });
      globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
        if (String(input).includes("/periods/current") && periodLookupPending) return new Promise<Response>(() => {});
        return mockApi(input, init);
      }) as typeof fetch;
      let periodLookupPending = false;
      const user = userEvent.setup();
      renderDashboard();
      await waitFor(() => expect(screen.getByText("May '26: $1.00 spent")).toBeInTheDocument());
      const summaryAlert = screen.getByText("Could not load Summary").closest('[role="alert"]');
      expect(summaryAlert).not.toBeNull();
      periodLookupPending = true;
      await user.click(within(summaryAlert as HTMLElement).getByRole("button", { name: "Retry" }));
      await user.click(screen.getByLabelText("12 months"));
      expect(screen.getByLabelText("Trends loading")).toBeInTheDocument();
      expect(screen.queryByText("May '26: $1.00 spent")).not.toBeInTheDocument();
      await user.click(screen.getByLabelText("Select breakdown chart"));
      await user.click(screen.getByRole("option", { name: "Repeated Expenses" }));
      expect(screen.getByText("Loading repeated expenses...")).toBeInTheDocument();
      expect(mockApi._calls.filter((call) => call.url.includes("/api/finance/spending/trends"))).toHaveLength(1);
      expect(mockApi._calls.filter((call) => call.url.includes("/api/expenses/suggestions"))).toHaveLength(0);
    });

    it("shows a pending tag retry while period verification is unresolved", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/summary": { status: 500, body: { code: "INTERNAL_SERVER_ERROR", message: "Summary failed" } },
        "/api/finance/spending/by-tag": { status: 500, body: { code: "INTERNAL_SERVER_ERROR", message: "Tag spending failed" } },
        "/api/finance/defaults": { body: { defaults: null } },
      });
      let resolvePeriod!: (value: Response) => void;
      let periodRequests = 0;
      globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
        if (String(input).includes("/periods/current")) {
          periodRequests += 1;
          if (periodRequests === 2) return new Promise<Response>((resolve) => { resolvePeriod = resolve; });
        }
        return mockApi(input, init);
      }) as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();
      const summaryAlert = await screen.findByText("Could not load Summary");
      const tagAlert = await screen.findByText("Could not load Tag spending");
      await user.click(within(summaryAlert.closest('[role="alert"]') as HTMLElement).getByRole("button", { name: "Retry" }));
      await user.click(within(tagAlert.closest('[role="alert"]') as HTMLElement).getByRole("button", { name: "Retry" }));
      expect(screen.getByLabelText("Tag spending loading")).toBeInTheDocument();
      expect(screen.queryByText("Could not load Tag spending")).not.toBeInTheDocument();
      expect(mockApi._calls.filter((call) => call.url.includes("/api/finance/spending/by-tag"))).toHaveLength(1);

      await act(async () => resolvePeriod(new Response(JSON.stringify({ code: "PERIOD_NOT_FOUND", message: "No period" }), { status: 404, headers: { "Content-Type": "application/json" } })));
      expect(await screen.findByText(/No budget configured yet/)).toBeInTheDocument();
      expect(screen.queryByText("Could not load Tag spending")).not.toBeInTheDocument();
      expect(mockApi._calls.filter((call) => call.url.includes("/api/finance/spending/by-tag"))).toHaveLength(1);
    });

    it("does not offer an unconfigured budget when defaults fail during period recovery", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": mockSequence([
          { body: { period: testPeriod } },
          { status: 404, body: { code: "PERIOD_NOT_FOUND", message: "No period" } },
          { status: 404, body: { code: "PERIOD_NOT_FOUND", message: "No period" } },
        ]),
        ...dashboardDataEmptyRoutes(),
        "/api/finance/summary": { status: 500, body: { code: "INTERNAL_SERVER_ERROR", message: "Summary failed" } },
        "/api/finance/defaults": mockSequence([
          { status: 500, body: { code: "INTERNAL_SERVER_ERROR", message: "Defaults unavailable" } },
          { body: { defaults: null } },
        ]),
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();
      const summaryAlert = await screen.findByText("Could not load Summary");
      await user.click(within(summaryAlert.closest('[role="alert"]') as HTMLElement).getByRole("button", { name: "Retry" }));

      expect(await screen.findByText("Could not load the dashboard")).toBeInTheDocument();
      expect(screen.queryByText(/No budget configured yet/)).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /Create .* Period/ })).not.toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Retry" }));
      expect(await screen.findByText(/No budget configured yet/)).toBeInTheDocument();
    });

    it("recovers the previous trend window after a wider-window error", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/spending/trends": mockSequence([
          { body: { trends: [{ year: 2026, month: 5, totalSpent: 100, budgetAmount: 300000, essentialsSpent: 50, desiresSpent: 50, savingsSpent: 0, essentialsPercent: 50, desiresPercent: 30, savingsPercent: 20 }] } },
          { status: 500, body: { code: "INTERNAL_SERVER_ERROR", message: "Trend window failed" } },
          { body: { trends: [{ year: 2026, month: 5, totalSpent: 200, budgetAmount: 300000, essentialsSpent: 100, desiresSpent: 100, savingsSpent: 0, essentialsPercent: 50, desiresPercent: 30, savingsPercent: 20 }] } },
        ]),
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => expect(screen.getAllByText("Monthly Spending").length).toBeGreaterThanOrEqual(2));
      await user.click(screen.getByLabelText("12 months"));
      await waitFor(() => expect(screen.getByText("Could not load Trends")).toBeInTheDocument());

      await user.click(screen.getByLabelText("6 months"));
      await waitFor(() => expect(screen.getAllByText("Monthly Spending").length).toBeGreaterThanOrEqual(2));
      expect(mockApi._calls.filter((call) => call.url.includes("/api/finance/spending/trends") && call.url.includes("months=6")).length).toBeGreaterThanOrEqual(2);
    });

    it("renders an explicit empty state for a health trend with no points", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/health-score/trend": { body: { trends: [] } },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("No health score trend is available.")).toBeInTheDocument();
      });
    });

    it("renders summary bar with budget values", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Total Budget")).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(screen.getAllByText("$3,000.00")).toHaveLength(2);
      });
      expect(screen.getByText("Total Spent")).toBeInTheDocument();
      expect(screen.getByText("$0.00")).toBeInTheDocument();
      expect(screen.getByText("Remaining")).toBeInTheDocument();
      expect(screen.getByText("Days Left")).toBeInTheDocument();
    });

    it("renders empty state with CTA to log expense", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("No expenses yet")).toBeInTheDocument();
      });

      const ctaLink = screen.getByRole("link", {
        name: /log your first expense/i,
      });
      expect(ctaLink).toBeInTheDocument();
      expect(ctaLink).toHaveAttribute("href", "/expenses/new");
    });

    it("does not expose outline metadata for dashboard sections without rendered content", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("No expenses yet")).toBeInTheDocument();
      });

      expect(document.querySelector('[data-outline-title="Historical Comparison"]')).toBeNull();
      expect(document.querySelector('[data-outline-title="Trends"]')).not.toBeNull();
      expect(document.querySelector('[data-outline-title="Cumulative Spending"]')).toBeNull();
    });

    it("keeps repeated expenses available when tag spending fails", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
        "/api/finance/spending/by-tag": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Tags failed" },
        },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument());
      await user.click(screen.getByLabelText("Select breakdown chart"));
      await user.click(await screen.findByRole("option", { name: "Repeated Expenses" }));

      await waitFor(() => expect(screen.getByText(/Frequency shows how often/i)).toBeInTheDocument());
      await user.click(screen.getByLabelText("Select breakdown chart"));
      await user.click(await screen.findByRole("option", { name: "Spending by Tag" }));
      expect(screen.getByText("Could not load Tag spending")).toBeInTheDocument();
    });

    it("renders repeated-expenses chart with frequency and recency context", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      // Wait for the Breakdown section to render with default "Spending by Tag"
      await waitFor(() => {
        expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument();
      });

      const breakdownTrigger = screen.getByLabelText("Select breakdown chart");
      await user.click(breakdownTrigger);
      const repeatedOption = await screen.findByRole("option", { name: "Repeated Expenses" });
      await user.click(repeatedOption);

      await waitFor(() => {
        expect(screen.getByText(/Frequency shows how often/i)).toBeInTheDocument();
      });

      expect(screen.getAllByText("Groceries").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Coffee").length).toBeGreaterThan(0);
      expect(screen.getByLabelText("Recency legend")).toHaveTextContent("Today");
      expect(screen.getByLabelText("Recency legend")).toHaveTextContent("Last 7 days");
      expect(screen.getByLabelText("Recency legend")).toHaveTextContent("Last 30 days");
      expect(screen.getByLabelText("Recency legend")).not.toHaveTextContent("Older");
      expect(screen.getByLabelText("Repeated expense details")).toHaveTextContent(
        "Groceries: Frequency 114, Recency Last 7 days",
      );
      expect(
        mockApi._calls.some((call) =>
          call.url.includes("/api/expenses/suggestions?page=1&pageSize=10"),
        ),
      ).toBe(true);
    });

    it("shows a local repeated-expenses empty state without changing dashboard empty expense behavior", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("No expenses yet")).toBeInTheDocument();
      });

      const breakdownTrigger = screen.getByLabelText("Select breakdown chart");
      await user.click(breakdownTrigger);
      const repeatedOption = await screen.findByRole("option", { name: "Repeated Expenses" });
      await user.click(repeatedOption);

      await waitFor(() => {
        expect(
          screen.getByText(/Not enough expense history yet/i),
        ).toBeInTheDocument();
      });
    });

    it("keeps other dashboard sections rendering when repeated-expenses fetch fails", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
        "/api/expenses/suggestions": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Suggestions failed" },
        },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("Recent Expenses").length).toBeGreaterThanOrEqual(1);
      });

      const breakdownTrigger = screen.getByLabelText("Select breakdown chart");
      await user.click(breakdownTrigger);
      const repeatedOption = await screen.findByRole("option", { name: "Repeated Expenses" });
      await user.click(repeatedOption);

      await waitFor(() => {
        expect(
          screen.getByText("Repeated expenses are unavailable right now."),
        ).toBeInTheDocument();
      });
      expect(screen.queryByText("Suggestions failed")).not.toBeInTheDocument();
      const errorMessage = screen.getByText("Repeated expenses are unavailable right now.");
      const retryButton = errorMessage.parentElement?.querySelector("button");
      expect(retryButton).not.toBeNull();
      const requestCount = mockApi._calls.filter((call) => call.url.includes("/api/expenses/suggestions")).length;
      await user.click(retryButton as HTMLElement);
      await waitFor(() => {
        expect(mockApi._calls.filter((call) => call.url.includes("/api/expenses/suggestions")).length).toBeGreaterThan(requestCount);
      });
    });

    it("refreshes selected suggestions with Refresh all data", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();
      await waitFor(() => expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument());
      await user.click(screen.getByLabelText("Select breakdown chart"));
      await user.click(await screen.findByRole("option", { name: "Repeated Expenses" }));
      await waitFor(() => expect(screen.getByText(/Frequency shows how often/i)).toBeInTheDocument());
      const requestCount = mockApi._calls.filter((call) => call.url.includes("/api/expenses/suggestions")).length;
      await user.click(screen.getByRole("button", { name: "Refresh all data" }));
      await waitFor(() => {
        expect(mockApi._calls.filter((call) => call.url.includes("/api/expenses/suggestions")).length).toBeGreaterThan(requestCount);
      });
    });

    it("displays currency symbol and precision from the period", async () => {
      const jpyPeriod = buildPeriod({
        ...testPeriod,
        budgetAmount: 300000,
        reportingCurrencyCode: "JPY",
      });
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: jpyPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard(buildUser({ ...testUser, currency: "EUR" }));

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(screen.getAllByText("¥300,000")).toHaveLength(2);
      });
      expect(screen.getByText("¥0")).toBeInTheDocument();
      expect(screen.queryByText("€3,000.00")).not.toBeInTheDocument();
    });

    it("adopts changed period metadata returned by Refresh all data", async () => {
      const refreshedPeriod = buildPeriod({
        ...testPeriod,
        budgetAmount: 450000,
        updatedAt: "2026-05-02T00:00:00Z",
      });
      const refreshedSummary = buildPeriodSummary({
        ...testSummary,
        periodId: refreshedPeriod.id,
        totalBudget: 450000,
        totalSpent: 120000,
        remaining: 330000,
      });
      const mockApi = createMockApi({
        "/api/finance/periods/current": mockSequence([
          { body: { period: testPeriod } },
          { body: { period: refreshedPeriod } },
        ]),
        ...dashboardDataEmptyRoutes(),
        "/api/finance/summary": mockSequence([
          { body: { summary: testSummary } },
          { body: { summary: refreshedSummary } },
        ]),
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;
      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => expect(screen.getByText("$3,000.00")).toBeInTheDocument());
      await user.click(screen.getByRole("button", { name: "Refresh all data" }));

      await waitFor(() => {
        expect(screen.getByText("$4,500.00")).toBeInTheDocument();
        expect(screen.getByText("$1,200.00")).toBeInTheDocument();
      });
    });

    it("color-codes remaining balance green when > 30%", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("$3,000.00").length).toBeGreaterThan(0);
      });

      const amounts = screen.getAllByText("$3,000.00");
      const greenAmount = amounts.find((element) =>
        element.className.includes("text-green"),
      );
      expect(greenAmount).toBeDefined();
    });

    it("shows no color class when budget is $0", async () => {
      const zeroBudgetPeriod = buildPeriod({ ...testPeriod, budgetAmount: 0 });
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: zeroBudgetPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              ...testSummary,
              totalBudget: 0,
              totalSpent: 0,
              remaining: 0,
              essentials: { allocated: 0, spent: 0, remaining: 0, percentUsed: 0 },
              desires: { allocated: 0, spent: 0, remaining: 0, percentUsed: 0 },
              savings: { allocated: 0, spent: 0, remaining: 0, percentUsed: 0 },
            }),
          },
        },
        "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
        "/api/finance/spending/cumulative": { body: { points: [] } },
        "/api/expenses": { body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false } },
        "/api/finance/spending/comparison": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
        },
        "/api/finance/prorata/upcoming": { body: { schedules: [] } },
        "/api/finance/spending/trends": { body: { trends: [] } },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("$0.00").length).toBeGreaterThan(0);
      });

      const zeroAmounts = screen.getAllByText("$0.00");
      zeroAmounts.forEach((element) => {
        expect(element.className).not.toContain("text-green");
        expect(element.className).not.toContain("text-yellow");
        expect(element.className).not.toContain("text-red");
      });
    });
  });
});
