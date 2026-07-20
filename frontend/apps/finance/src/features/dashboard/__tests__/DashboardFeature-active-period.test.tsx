import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { buildUser, buildPeriod, buildPeriodSummary, createMockApi } from "@gofin/test-utils";
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
  });

  describe("active period exists", () => {
    it("renders summary bar with budget values", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      expect(screen.getByText("Total Budget")).toBeInTheDocument();
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
      expect(document.querySelector('[data-outline-title="Trends"]')).toBeNull();
      expect(document.querySelector('[data-outline-title="Cumulative Spending"]')).toBeNull();
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
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
        "/api/expenses/suggestions": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Suggestions failed" },
        },
      }) as unknown as typeof fetch;
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
    });

    it("displays currency symbol from user profile", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard(buildUser({ ...testUser, currency: "EUR" }));

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(screen.getAllByText("€3,000.00")).toHaveLength(2);
      });
      expect(screen.getByText("€0.00")).toBeInTheDocument();
    });

    it("color-codes remaining balance green when > 30%", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
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
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
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
