import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DashboardFeature } from "../index";
import {
  buildUser,
  buildPeriod,
  buildPeriodSummary,
  buildDefaults,
  buildExpense,
  createMockApi,
  renderWithRouter,
} from "@gofin/test-utils";

// --- Shared test data built from factories ---

const testUser = buildUser({
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  currency: "USD",
});

const testPeriod = buildPeriod({
  id: "period-abc",
  userId: "user-1",
  year: 2026,
  month: 5,
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
});

const testDefaults = buildDefaults({
  userId: "user-1",
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
});

const testSummary = buildPeriodSummary({
  periodId: "period-abc",
  year: 2026,
  month: 5,
  totalBudget: 300000,
  totalSpent: 54500,
  remaining: 245500,
  daysInPeriod: 31,
  daysElapsed: 3,
  dailySpendRate: 18166,
  budgetPace: 8767,
  isOnTrack: false,
  essentials: { allocated: 150000, spent: 50000, remaining: 100000, percentUsed: 33.33 },
  desires: { allocated: 90000, spent: 4500, remaining: 85500, percentUsed: 5.0 },
  savings: { allocated: 60000, spent: 0, remaining: 60000, percentUsed: 0.0 },
});

const testTagSpending = [
  { tagId: "tag-food", tagName: "Food", amount: 50000, percentOfTotal: 91.74 },
  { tagId: "tag-social", tagName: "Social", amount: 4500, percentOfTotal: 8.26 },
];

const testCumulativeData = Array.from({ length: 31 }, (_, index) => ({
  day: index + 1,
  actual: index < 3 ? (index + 1) * 18166 : 54500,
  ideal: Math.round((300000 / 31) * (index + 1)),
}));

const testExpenseSuggestions = [
  {
    name: "Groceries",
    amount: 50000,
    currency: "USD",
    expenseType: "essentials" as const,
    tagId: "tag-food",
    frequency: 114,
    lastUsedAt: "2026-05-02T10:00:00Z",
    recencyBucket: "last_7_days" as const,
    frecencyScore: 145,
  },
  {
    name: "Coffee",
    amount: 4500,
    currency: "USD",
    expenseType: "desires" as const,
    tagId: "tag-social",
    frequency: 42,
    lastUsedAt: "2026-05-01T09:00:00Z",
    recencyBucket: "today" as const,
    frecencyScore: 90,
  },
];

const testExpenses = [
  buildExpense({
    id: "exp-1",
    userId: "user-1",
    name: "Groceries",
    amount: 50000,
    currency: "USD",
    expenseType: "essentials",
    tagId: "tag-food",
    expenseDate: "2026-05-02",
    periodYear: 2026,
    periodMonth: 5,
    createdAt: "2026-05-02T10:00:00Z",
  }),
  buildExpense({
    id: "exp-2",
    userId: "user-1",
    name: "Coffee",
    amount: 4500,
    currency: "USD",
    expenseType: "desires",
    tagId: "tag-social",
    expenseDate: "2026-05-01",
    periodYear: 2026,
    periodMonth: 5,
    createdAt: "2026-05-01T09:00:00Z",
  }),
];

// --- URL-based mock API route sets ---

/** Standard dashboard data responses for an active period with no expenses. */
function dashboardDataEmptyRoutes() {
  return {
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
          essentials: { allocated: 150000, spent: 0, remaining: 150000, percentUsed: 0 },
          desires: { allocated: 90000, spent: 0, remaining: 90000, percentUsed: 0 },
          savings: { allocated: 60000, spent: 0, remaining: 60000, percentUsed: 0 },
        }),
      },
    },
    "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
    "/api/finance/spending/cumulative": { body: { points: [] } },
    "/api/expenses/suggestions": {
      body: { data: [], total: 0, page: 1, pageSize: 10, hasMore: false },
    },
    "/api/expenses": { body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false } },
    "/api/finance/spending/comparison": {
      status: 404,
      body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
    },
    "/api/finance/prorata/upcoming": { body: { schedules: [] } },
    "/api/finance/spending/trends": { body: { trends: [] } },
  };
}

/** Standard dashboard data responses for an active period with expenses. */
function dashboardDataWithExpensesRoutes() {
  return {
    "/api/finance/summary": { body: { summary: testSummary } },
    "/api/finance/spending/by-tag": { body: { tagSpending: testTagSpending } },
    "/api/finance/spending/cumulative": { body: { points: testCumulativeData } },
    "/api/expenses/suggestions": {
      body: { data: testExpenseSuggestions, total: 2, page: 1, pageSize: 10, hasMore: false },
    },
    "/api/expenses": {
      body: { data: testExpenses, total: 2, page: 1, pageSize: 5, hasMore: false },
    },
    "/api/finance/spending/comparison": {
      body: {
        comparison: {
          currentSpent: 54500,
          previousSpent: 48000,
          rollingAverage: null,
          changePercent: 13.54,
        },
      },
    },
    "/api/finance/prorata/upcoming": { body: { schedules: [] } },
    "/api/finance/spending/trends": { body: { trends: [] } },
  };
}

// --- Render helper ---

function renderDashboard(user = testUser) {
  return renderWithRouter(<DashboardFeature user={user} />, { route: "/dashboard" });
}

// --- Tests ---

describe("DashboardFeature", () => {
  beforeEach(() => {
    // Reset globalThis.fetch before each test; each test sets its own createMockApi
  });

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

      // Switch to Repeated Expenses via the breakdown Select
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

      // Switch to Repeated Expenses via the breakdown Select
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

      // Switch to Repeated Expenses via the breakdown Select
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

  describe("no period exists (PERIOD_NOT_FOUND)", () => {
    it("shows creation prompt with default values", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const budgetInput = screen.getByLabelText("Monthly Budget") as HTMLInputElement;
      expect(budgetInput.value).toBe("3000");

      const essentialsInput = screen.getByLabelText("Essentials %") as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");

      const desiresInput = screen.getByLabelText("Desires %") as HTMLInputElement;
      expect(desiresInput.value).toBe("30");

      const savingsInput = screen.getByLabelText("Savings %") as HTMLInputElement;
      expect(savingsInput.value).toBe("20");
    });

    it("shows zero-budget warning when default budget is $0", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
        },
        "/api/finance/defaults": {
          body: { defaults: buildDefaults({ ...testDefaults, budgetAmount: 0 }) },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/no budget configured/i)).toBeInTheDocument();
      });
    });

    it("uses fallback defaults when defaults endpoint fails", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
        },
        "/api/finance/defaults": {
          status: 404,
          body: { code: "NOT_FOUND", message: "Default settings not found" },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const essentialsInput = screen.getByLabelText("Essentials %") as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");
    });

    it("renders CreatePeriodPrompt with null defaults when defaults fetch returns server error", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
        },
        "/api/finance/defaults": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Database connection failed" },
        },
      }) as unknown as typeof fetch;
      renderDashboard();

      // When defaults fetch fails with a server error, the page should still render
      // the creation prompt using fallback defaults (50/30/20, $0 budget)
      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      // Fallback budget is empty (renders as empty string because amount is 0)
      const budgetInput = screen.getByLabelText("Monthly Budget") as HTMLInputElement;
      expect(budgetInput.value).toBe("");

      // Fallback split percentages are 50/30/20
      const essentialsInput = screen.getByLabelText("Essentials %") as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");

      const desiresInput = screen.getByLabelText("Desires %") as HTMLInputElement;
      expect(desiresInput.value).toBe("30");

      const savingsInput = screen.getByLabelText("Savings %") as HTMLInputElement;
      expect(savingsInput.value).toBe("20");
    });

    it("validates E/D/S split sums to 100%", async () => {
      globalThis.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
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
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
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
              essentials: { allocated: 150000, spent: 0, remaining: 150000, percentUsed: 0 },
              desires: { allocated: 90000, spent: 0, remaining: 90000, percentUsed: 0 },
              savings: { allocated: 60000, spent: 0, remaining: 60000, percentUsed: 0 },
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
    });

    it("does NOT change stored defaults when user overrides values", async () => {
      const mockApi = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found for 2026-05" },
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
        "/api/expenses": { body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false } },
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
      expect((createCall!.body as { budgetAmount: number }).budgetAmount).toBe(500000);

      // Verify no PUT to /api/finance/defaults was made
      const defaultsUpdateCall = mockApi._calls.find(
        (call) =>
          call.url.includes("/api/finance/defaults") &&
          call.method === "PUT",
      );
      expect(defaultsUpdateCall).toBeUndefined();
    });
  });

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

      // Essentials gauge
      const essentialsGauge = screen.getByTestId("gauge-essentials");
      expect(within(essentialsGauge).getByText("Essentials")).toBeInTheDocument();
      expect(within(essentialsGauge).getByText("33%")).toBeInTheDocument();
      expect(within(essentialsGauge).getByText(/\$500\.00 of \$1,500\.00/)).toBeInTheDocument();

      // Desires gauge
      const desiresGauge = screen.getByTestId("gauge-desires");
      expect(within(desiresGauge).getByText("Desires")).toBeInTheDocument();
      expect(within(desiresGauge).getByText("5%")).toBeInTheDocument();

      // Savings gauge
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
