import { describe, it, expect } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  buildDefaults,
  buildPeriod,
  buildPeriodSummary,
  buildUser,
  createMockApi,
  mockSequence,
} from "@gofin/test-utils";
import { renderDashboard } from "./render";
import { dashboardDataEmptyRoutes, testDefaults, testPeriod } from "./fixtures";

describe("DashboardFeature", () => {
  describe("no period exists (PERIOD_NOT_FOUND)", () => {
    it("shows creation prompt with default values", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const budgetInput = screen.getByLabelText(
        "Monthly Budget",
      ) as HTMLInputElement;
      expect(budgetInput.value).toBe("3000");

      const essentialsInput = screen.getByLabelText(
        "Essentials %",
      ) as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");

      const desiresInput = screen.getByLabelText(
        "Desires %",
      ) as HTMLInputElement;
      expect(desiresInput.value).toBe("30");

      const savingsInput = screen.getByLabelText(
        "Savings %",
      ) as HTMLInputElement;
      expect(savingsInput.value).toBe("20");

      const currencySelect = screen.getByLabelText(
        "Reporting Currency",
      ) as HTMLSelectElement;
      expect(currencySelect.value).toBe("USD");
      expect(
        screen.getByText(/reporting currency cannot be changed/i),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /default currency changes only apply to future periods/i,
        ),
      ).toBeInTheDocument();
    });

    it("shows zero-budget warning when default budget is $0", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": {
          body: {
            defaults: buildDefaults({ ...testDefaults, budgetAmount: 0 }),
          },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/no budget configured/i)).toBeInTheDocument();
      });
    });

    it("offers retry instead of a create form when defaults are missing", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": {
          status: 404,
          body: { code: "NOT_FOUND", message: "Default settings not found" },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      expect(
        await screen.findByText("Something went wrong"),
      ).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
      expect(screen.queryByLabelText("Essentials %")).not.toBeInTheDocument();
    });

    it("blocks the create form when defaults fetch returns a server error", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": {
          status: 500,
          body: {
            code: "INTERNAL_SERVER_ERROR",
            message: "Database connection failed",
          },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      expect(
        await screen.findByText("Something went wrong"),
      ).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
      expect(screen.queryByLabelText("Monthly Budget")).not.toBeInTheDocument();
    });

    it("leaves reporting currency empty when no usable default or user currency", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: null } },
      }) as unknown as typeof fetch;
      renderDashboard(buildUser({ currency: "ZZZ" }));

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const currencySelect = screen.getByLabelText(
        "Reporting Currency",
      ) as HTMLSelectElement;
      expect(currencySelect.value).toBe("");
      expect(
        screen.getByRole("option", { name: "Select a currency" }),
      ).toBeInTheDocument();
    });

    it("requires a reporting currency and clears the error on selection", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: null } },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;

      const user = userEvent.setup();
      renderDashboard(buildUser({ currency: "" }));

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      expect(
        screen.getByText(/reporting currency is required/i),
      ).toBeInTheDocument();
      const createCall = mockApi._calls.find(
        (call) =>
          call.url.includes("/api/finance/periods") &&
          !call.url.includes("/current") &&
          call.method === "POST",
      );
      expect(createCall).toBeUndefined();

      const currencySelect = screen.getByLabelText(
        "Reporting Currency",
      ) as HTMLSelectElement;
      fireEvent.change(currencySelect, { target: { value: "USD" } });

      expect(
        screen.queryByText(/reporting currency is required/i),
      ).not.toBeInTheDocument();
    });

    it("validates E/D/S split sums to 100%", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
      }) as unknown as typeof fetch;

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const savingsInput = screen.getByLabelText("Savings %");
      await user.clear(savingsInput);
      await user.type(savingsInput, "19");

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      expect(
        screen.getByText(/percentages must sum to 100%/i),
      ).toBeInTheDocument();
    });

    it("creates period on valid submission and shows dashboard", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": { status: 201, body: { period: testPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              periodId: "period-abc",
              year: 2026,
              month: 5,
              totalBudget: 300000,
              totalSpent: 0,
              remaining: 300000,
              daysInPeriod: 31,
              daysElapsed: 3,
              dailySpendRate: 0,
              budgetPace: 9677,
              isOnTrack: true,
              essentials: {
                allocated: 150000,
                spent: 0,
                remaining: 150000,
                percentUsed: 0,
              },
              desires: {
                allocated: 90000,
                spent: 0,
                remaining: 90000,
                percentUsed: 0,
              },
              savings: {
                allocated: 60000,
                spent: 0,
                remaining: 60000,
                percentUsed: 0,
              },
            }),
          },
        },
        "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
        "/api/finance/spending/cumulative": { body: { points: [] } },
        "/api/expenses": {
          body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false },
        },
        "/api/finance/spending/comparison": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
        },
        "/api/finance/prorata/upcoming": { body: { schedules: [] } },
        "/api/finance/spending/trends": { body: { trends: [] } },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
        expect(screen.getAllByText("$3,000.00")).toHaveLength(2);
      });

      const createCall = mockApi._calls.find(
        (call) =>
          call.url.includes("/api/finance/periods") &&
          !call.url.includes("/current") &&
          call.method === "POST",
      );
      expect(
        (createCall!.body as { reportingCurrencyCode: string })
          .reportingCurrencyCode,
      ).toBe("USD");
    });

    it("recovers after Refresh all reports a missing period and the period is recreated", async () => {
      const recreatedPeriod = buildPeriod({
        ...testPeriod,
        id: "period-recreated",
        budgetAmount: 360000,
        updatedAt: "2026-05-03T00:00:00Z",
      });
      const mockApi = createMockApi({
        "/api/finance/periods/current": mockSequence([
          { body: { period: testPeriod } },
          {
            status: 404,
            body: {
              code: "PERIOD_NOT_FOUND",
              message: "No budget period found",
            },
          },
        ]),
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": {
          status: 201,
          body: { period: recreatedPeriod },
        },
        ...dashboardDataEmptyRoutes(),
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              periodId: recreatedPeriod.id,
              year: recreatedPeriod.year,
              month: recreatedPeriod.month,
              totalBudget: recreatedPeriod.budgetAmount,
              totalSpent: 0,
              remaining: recreatedPeriod.budgetAmount,
            }),
          },
        },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;

      const user = userEvent.setup();
      renderDashboard();
      await waitFor(() =>
        expect(screen.getByText("Dashboard")).toBeInTheDocument(),
      );

      await user.click(
        screen.getByRole("button", { name: "Refresh all data" }),
      );
      await waitFor(() =>
        expect(screen.getByText(/set up/i)).toBeInTheDocument(),
      );

      await user.click(screen.getByRole("button", { name: /create/i }));
      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
        expect(screen.getAllByText("$3,600.00")).toHaveLength(2);
      });
    });

    it("sends the selected reporting currency and parses amount with its precision", async () => {
      const jpyPeriod = buildPeriod({
        ...testPeriod,
        budgetAmount: 300000,
        reportingCurrencyCode: "JPY",
      });
      const mockApi = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": { status: 201, body: { period: jpyPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              periodId: "period-abc",
              year: 2026,
              month: 5,
              totalBudget: 300000,
              totalSpent: 0,
              remaining: 300000,
              isOnTrack: true,
            }),
          },
        },
        "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
        "/api/finance/spending/cumulative": { body: { points: [] } },
        "/api/expenses": {
          body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false },
        },
        "/api/finance/spending/comparison": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
        },
        "/api/finance/prorata/upcoming": { body: { schedules: [] } },
        "/api/finance/spending/trends": { body: { trends: [] } },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const currencySelect = screen.getByLabelText(
        "Reporting Currency",
      ) as HTMLSelectElement;
      fireEvent.change(currencySelect, { target: { value: "JPY" } });

      const budgetInput = screen.getByLabelText(
        "Monthly Budget",
      ) as HTMLInputElement;
      expect(budgetInput.step).toBe("1");
      await user.clear(budgetInput);
      await user.type(budgetInput, "300000");

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
        expect(screen.getAllByText("¥300,000")).toHaveLength(2);
      });

      const createCall = mockApi._calls.find(
        (call) =>
          call.url.includes("/api/finance/periods") &&
          !call.url.includes("/current") &&
          call.method === "POST",
      );
      expect(createCall?.body).toMatchObject({
        budgetAmount: 300000,
        reportingCurrencyCode: "JPY",
      });
    });

    it("does NOT change stored defaults when user overrides values", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: {
            code: "PERIOD_NOT_FOUND",
            message: "No budget period found for 2026-05",
          },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": { status: 201, body: { period: testPeriod } },
        "/api/finance/summary": {
          body: {
            summary: buildPeriodSummary({
              periodId: "period-abc",
              year: 2026,
              month: 5,
              totalBudget: 300000,
              totalSpent: 0,
              remaining: 300000,
              isOnTrack: true,
            }),
          },
        },
        "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
        "/api/finance/spending/cumulative": { body: { points: [] } },
        "/api/expenses": {
          body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false },
        },
        "/api/finance/spending/comparison": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
        },
        "/api/finance/prorata/upcoming": { body: { schedules: [] } },
        "/api/finance/spending/trends": { body: { trends: [] } },
      });
      globalThis.fetch = mockApi as unknown as typeof fetch;

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const budgetInput = screen.getByLabelText("Monthly Budget");
      await user.clear(budgetInput);
      await user.type(budgetInput, "5000");

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      // Verify the POST to /api/finance/periods had the right budget
      const createCall = mockApi._calls.find(
        (call) =>
          call.url.includes("/api/finance/periods") &&
          !call.url.includes("/current") &&
          call.method === "POST",
      );
      expect(createCall).toBeDefined();
      expect((createCall!.body as { budgetAmount: number }).budgetAmount).toBe(
        500000,
      );
      expect(
        (createCall!.body as { reportingCurrencyCode: string })
          .reportingCurrencyCode,
      ).toBe("USD");

      // Verify no PUT to /api/finance/defaults was made
      const defaultsUpdateCall = mockApi._calls.find(
        (call) =>
          call.url.includes("/api/finance/defaults") && call.method === "PUT",
      );
      expect(defaultsUpdateCall).toBeUndefined();
    });
  });
});
