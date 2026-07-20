import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMockApi } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import { testPeriod, dashboardDataEmptyRoutes } from "./fixtures";

describe("DashboardFeature", () => {
  describe("error state", () => {
    it("renders error state on server error", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Database connection failed" },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Something went wrong")).toBeInTheDocument();
      });

      expect(
        screen.getByRole("button", { name: /retry/i }),
      ).toBeInTheDocument();
    });

    it("retries fetch on retry button click", async () => {
      // First call returns error, subsequent calls return success.
      // Use mockSequence-like behavior: swap fetch after error is shown.
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Temporary failure" },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Something went wrong")).toBeInTheDocument();
      });

      // Now swap to a successful mock for retry
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
        ...dashboardDataEmptyRoutes(),
      }) as unknown as typeof fetch;

      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: /retry/i }));

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });
    });
  });

  describe("dashboard data error", () => {
    it("does not crash when dashboard data fetch fails", async () => {
      // Period is found, but all data fetches fail (network error)
      // Use a mock that returns period for the initial call, then rejects all others
      const mockApi = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
      });
      // Override: after period found, remaining URLs will hit "No mock route" error
      // which simulates network failures
      globalThis.fetch = mockApi as unknown as typeof fetch;
      renderDashboard();

      // Dashboard header should still render even with data errors
      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });
    });
  });
});
