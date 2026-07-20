import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { createMockApi } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import { testPeriod, dashboardDataWithExpensesRoutes } from "./fixtures";

describe("DashboardFeature", () => {
  describe("recent expenses", () => {
    it("shows recent expenses when expenses exist", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("Recent Expenses").length).toBeGreaterThanOrEqual(1);
      });

      expect(screen.getAllByText("Groceries").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Coffee").length).toBeGreaterThan(0);
      expect(screen.getByText("$500.00")).toBeInTheDocument();
      expect(screen.getByText("$45.00")).toBeInTheDocument();
    });

    it("shows View All link to /expenses", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("Recent Expenses").length).toBeGreaterThanOrEqual(1);
      });

      const viewAllLink = screen.getByRole("link", { name: /view all/i });
      expect(viewAllLink).toHaveAttribute("href", "/expenses");
    });

    it("updates total spent in summary bar from summary endpoint", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getAllByText("Recent Expenses").length).toBeGreaterThanOrEqual(1);
      });

      // totalSpent from summary: 54500 cents = $545.00
      await waitFor(() => {
        expect(screen.getAllByText("$545.00").length).toBeGreaterThanOrEqual(1);
      });

      // Remaining: $3,000.00 - $545.00 = $2,455.00
      expect(screen.getByText("$2,455.00")).toBeInTheDocument();
    });
  });
});
