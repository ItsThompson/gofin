import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { createMockApi } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import { testPeriod, dashboardDataEmptyRoutes, dashboardDataWithExpensesRoutes } from "./fixtures";

describe("DashboardFeature", () => {
  describe("responsive layout", () => {
    it("shows Log Expense button on mobile viewport", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      const logExpenseLinks = screen.getAllByRole("link", { name: /log expense/i });
      expect(logExpenseLinks.length).toBeGreaterThanOrEqual(1);
    });

    it("charts container has hidden md:block class for mobile hiding", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataWithExpensesRoutes(),
      }) as unknown as typeof fetch;
      const { container } = renderDashboard();

      await waitFor(() => {
        expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument();
      });

      // Charts are wrapped in a hidden md:block container
      const hiddenCharts = container.querySelector(".hidden.md\\:block");
      expect(hiddenCharts).not.toBeNull();
      // Spending Pace + Historical Comparison are in a hidden md:grid container
      const gridContainer = container.querySelector(".hidden.md\\:grid");
      expect(gridContainer).not.toBeNull();
    });
  });
});
