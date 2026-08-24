import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMockApi } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import {
  testPeriod,
  testSummary,
  testTagSpending,
  testCumulativeData,
  testExpenses,
  dashboardDataWithExpensesRoutes,
} from "./fixtures";

describe("DashboardFeature", () => {
  describe("budget settings editor", () => {
    it("shows budget settings editor when gear button is clicked", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      const settingsButton = screen.getByLabelText("Budget Settings");
      await userEvent.click(settingsButton);

      expect(screen.getByTestId("budget-settings-editor")).toBeInTheDocument();
      expect(screen.getByLabelText("Monthly Budget")).toBeInTheDocument();
      expect(screen.getByLabelText("Essentials %")).toBeInTheDocument();
      expect(screen.getByLabelText("Desires %")).toBeInTheDocument();
      expect(screen.getByLabelText("Savings %")).toBeInTheDocument();
    });

    it("hides editor when cancel is clicked", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      await userEvent.click(screen.getByLabelText("Budget Settings"));
      expect(screen.getByTestId("budget-settings-editor")).toBeInTheDocument();

      await userEvent.click(screen.getByText("Cancel"));
      expect(screen.queryByTestId("budget-settings-editor")).not.toBeInTheDocument();
    });

    it("validates E/D/S split sums to 100% on save", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      await userEvent.click(screen.getByLabelText("Budget Settings"));

      const savingsInput = screen.getByLabelText("Savings %");
      await userEvent.clear(savingsInput);
      await userEvent.type(savingsInput, "19");

      await userEvent.click(screen.getByText("Save Changes"));

      await waitFor(() => {
        expect(screen.getByText(/must sum to 100%/i)).toBeInTheDocument();
      });
    });
  });

  describe("historical comparison widget", () => {
    it("shows historical comparison data with change indicator", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByTestId("historical-comparison")).toBeInTheDocument();
      });

      expect(screen.getAllByText("Historical Comparison").length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText("Current Period")).toBeInTheDocument();
      expect(screen.getByText("Previous Period")).toBeInTheDocument();
      expect(screen.getByText("$480.00")).toBeInTheDocument();
      expect(screen.getByText(/13\.5% from last period/)).toBeInTheDocument();
    });

    it("shows 'not enough data' when only one period exists", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        "/api/finance/summary": { body: { summary: testSummary } },
        "/api/finance/spending/by-tag": { body: { tagSpending: testTagSpending } },
        "/api/finance/spending/cumulative": { body: { points: testCumulativeData } },
        "/api/expenses": {
          body: { data: testExpenses, total: 2, page: 1, pageSize: 5, hasMore: false },
        },
        "/api/finance/spending/comparison": {
          body: {
            comparison: {
              currentSpent: 54500,
              previousSpent: 0,
              previousReportingCurrency: "",
              comparable: true,
              rollingAverage: null,
              changePercent: 0,
            },
          },
        },
        "/api/finance/prorata/upcoming": { body: { schedules: [] } },
        "/api/finance/spending/trends": { body: { trends: [] } },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByTestId("historical-comparison")).toBeInTheDocument();
      });

      expect(
        screen.getByText("Not enough data for comparison"),
      ).toBeInTheDocument();
    });
  });
});
