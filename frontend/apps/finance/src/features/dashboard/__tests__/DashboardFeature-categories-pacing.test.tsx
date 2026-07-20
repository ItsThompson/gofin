import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { buildPeriodSummary, createMockApi } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import { testPeriod, testSummary, dashboardDataWithExpensesRoutes } from "./fixtures";

describe("DashboardFeature", () => {
  describe("category gauges", () => {
    it("renders three category gauges with allocated and spent amounts", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByTestId("gauge-essentials")).toBeInTheDocument();
      });

      const essentialsGauge = screen.getByTestId("gauge-essentials");
      expect(within(essentialsGauge).getByText("Essentials")).toBeInTheDocument();
      expect(within(essentialsGauge).getByText("33%")).toBeInTheDocument();
      expect(within(essentialsGauge).getByText(/\$500\.00 of \$1,500\.00/)).toBeInTheDocument();

      const desiresGauge = screen.getByTestId("gauge-desires");
      expect(within(desiresGauge).getByText("Desires")).toBeInTheDocument();
      expect(within(desiresGauge).getByText("5%")).toBeInTheDocument();

      const savingsGauge = screen.getByTestId("gauge-savings");
      expect(within(savingsGauge).getByText("Savings")).toBeInTheDocument();
      expect(within(savingsGauge).getByText("0%")).toBeInTheDocument();
    });

    it("shows over-budget indicator when category exceeds allocation", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              ...testSummary,
              essentials: { allocated: 150000, spent: 200000, remaining: -50000, percentUsed: 133.33 },
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
        expect(screen.getByTestId("gauge-essentials")).toBeInTheDocument();
      });

      const essentialsGauge = screen.getByTestId("gauge-essentials");
      expect(within(essentialsGauge).getByText("133%")).toBeInTheDocument();
      expect(within(essentialsGauge).getByText(/over by/i)).toBeInTheDocument();
    });

    it("shows over-budget indicator for zero-allocation category with spending", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              ...testSummary,
              savings: { allocated: 0, spent: 5000, remaining: -5000, percentUsed: 150 },
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
        expect(screen.getByTestId("gauge-savings")).toBeInTheDocument();
      });

      const savingsGauge = screen.getByTestId("gauge-savings");
      expect(within(savingsGauge).getByText(/over by/i)).toBeInTheDocument();
    });
  });

  describe("pacing indicator", () => {
    it("renders pacing data from summary endpoint", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("Spending Pace").length).toBeGreaterThanOrEqual(1);
      });

      expect(screen.getByText("Daily Average")).toBeInTheDocument();
      expect(screen.getByText("Required Rate")).toBeInTheDocument();
      expect(screen.getByText("Status")).toBeInTheDocument();
    });

    it("shows on-track indicator when spending is under pace", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              ...testSummary,
              isOnTrack: true,
              totalSpent: 10000,
              remaining: 290000,
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
        expect(screen.getByText("On Track")).toBeInTheDocument();
      });
    });

    it("shows over-budget amount when totalSpent exceeds budget", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              ...testSummary,
              totalBudget: 100000,
              totalSpent: 150000,
              remaining: -50000,
              isOnTrack: false,
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
        expect(screen.getByText(/over by \$500\.00/i)).toBeInTheDocument();
      });
    });
  });
});
