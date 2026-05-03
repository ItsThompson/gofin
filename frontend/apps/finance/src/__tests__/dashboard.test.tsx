import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { DashboardPage } from "@/pages/DashboardPage";
import type { User } from "@gofin/types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockUser: User = {
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockPeriod = {
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
};

const mockDefaults = {
  userId: "user-1",
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

const mockSummary = {
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
};

const mockTagSpending = [
  { tagId: "tag-food", tagName: "Food", amount: 50000, percentOfTotal: 91.74 },
  { tagId: "tag-social", tagName: "Social", amount: 4500, percentOfTotal: 8.26 },
];

const mockCumulativeData = Array.from({ length: 31 }, (_, index) => ({
  day: index + 1,
  actual: index < 3 ? (index + 1) * 18166 : 54500,
  ideal: Math.round((300000 / 31) * (index + 1)),
}));

const mockExpenses = [
  {
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
    status: "active",
    isProRata: false,
    createdAt: "2026-05-02T10:00:00Z",
  },
  {
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
    status: "active",
    isProRata: false,
    createdAt: "2026-05-01T09:00:00Z",
  },
];

// --- Fetch mock helpers ---

function mockPeriodFound() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ period: mockPeriod }),
  });
}

function mockPeriodNotFound() {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 404,
    json: () =>
      Promise.resolve({
        code: "PERIOD_NOT_FOUND",
        message: "No budget period found for 2026-05",
      }),
  });
}

function mockDefaultsFound() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ defaults: mockDefaults }),
  });
}

function mockDefaultsNotFound() {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 404,
    json: () =>
      Promise.resolve({
        code: "NOT_FOUND",
        message: "Default settings not found",
      }),
  });
}

function mockServerError(message: string) {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 500,
    json: () =>
      Promise.resolve({
        code: "INTERNAL_SERVER_ERROR",
        message,
      }),
  });
}

/**
 * Mocks the 5 parallel dashboard data fetches after a period is found:
 * summary, spending/by-tag, spending/cumulative, expenses (recent 5), spending/comparison
 */
function mockDashboardDataEmpty() {
  // summary
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        summary: {
          ...mockSummary,
          totalSpent: 0,
          remaining: 300000,
          dailySpendRate: 0,
          budgetPace: 9677,
          isOnTrack: true,
          essentials: { allocated: 150000, spent: 0, remaining: 150000, percentUsed: 0 },
          desires: { allocated: 90000, spent: 0, remaining: 90000, percentUsed: 0 },
          savings: { allocated: 60000, spent: 0, remaining: 60000, percentUsed: 0 },
        },
      }),
  });
  // spending/by-tag
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ tagSpending: [] }),
  });
  // spending/cumulative
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ points: [] }),
  });
  // recent expenses
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
  });
  // spending/comparison (fails gracefully for empty dashboard)
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 404,
    json: () =>
      Promise.resolve({ code: "PERIOD_NOT_FOUND", message: "Not enough data" }),
  });
}

function mockDashboardDataWithExpenses() {
  // summary
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ summary: mockSummary }),
  });
  // spending/by-tag
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ tagSpending: mockTagSpending }),
  });
  // spending/cumulative
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ points: mockCumulativeData }),
  });
  // recent expenses
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        data: mockExpenses,
        total: 2,
        page: 1,
        pageSize: 5,
        hasMore: false,
      }),
  });
  // spending/comparison
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        comparison: {
          currentSpent: 54500,
          previousSpent: 48000,
          rollingAverage: null,
          changePercent: 13.54,
        },
      }),
  });
}

function mockCreatePeriodSuccess() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 201,
    json: () => Promise.resolve({ period: mockPeriod }),
  });
}

function renderDashboard(user: User = mockUser) {
  return render(
    <MemoryRouter>
      <DashboardPage user={user} />
    </MemoryRouter>,
  );
}

// --- Tests ---

describe("DashboardPage", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("renders skeleton loading state initially", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => {}));
    renderDashboard();
    // Skeleton renders data-slot="skeleton" elements instead of loading text
    const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  describe("active period exists", () => {
    it("renders summary bar with budget values", async () => {
      mockPeriodFound();
      mockDashboardDataEmpty();
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
      mockPeriodFound();
      mockDashboardDataEmpty();
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

    it("displays currency symbol from user profile", async () => {
      mockPeriodFound();
      mockDashboardDataEmpty();
      renderDashboard({ ...mockUser, currency: "EUR" });

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(screen.getAllByText("€3,000.00")).toHaveLength(2);
      });
      expect(screen.getByText("€0.00")).toBeInTheDocument();
    });

    it("color-codes remaining balance green when > 30%", async () => {
      mockPeriodFound();
      mockDashboardDataEmpty();
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
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            period: { ...mockPeriod, budgetAmount: 0 },
          }),
      });
      // Summary with $0 budget
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            summary: {
              ...mockSummary,
              totalBudget: 0,
              totalSpent: 0,
              remaining: 0,
              essentials: { allocated: 0, spent: 0, remaining: 0, percentUsed: 0 },
              desires: { allocated: 0, spent: 0, remaining: 0, percentUsed: 0 },
              savings: { allocated: 0, spent: 0, remaining: 0, percentUsed: 0 },
            },
          }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ tagSpending: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ points: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
      });
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
      mockPeriodNotFound();
      mockDefaultsFound();
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
    });

    it("shows zero-budget warning when default budget is $0", async () => {
      mockPeriodNotFound();
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            defaults: { ...mockDefaults, budgetAmount: 0 },
          }),
      });
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/no budget configured/i)).toBeInTheDocument();
      });
    });

    it("uses fallback defaults when defaults endpoint fails", async () => {
      mockPeriodNotFound();
      mockDefaultsNotFound();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const essentialsInput = screen.getByLabelText(
        "Essentials %",
      ) as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");
    });

    it("validates E/D/S split sums to 100%", async () => {
      mockPeriodNotFound();
      mockDefaultsFound();

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
      mockPeriodNotFound();
      mockDefaultsFound();
      mockCreatePeriodSuccess();
      mockDashboardDataEmpty();

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
      mockPeriodNotFound();
      mockDefaultsFound();
      mockCreatePeriodSuccess();
      mockDashboardDataEmpty();

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

      const createCall = mockFetch.mock.calls.find(
        (call) =>
          typeof call[0] === "string" &&
          call[0].includes("/api/finance/periods") &&
          call[1]?.method === "POST",
      );
      expect(createCall).toBeDefined();
      const body = JSON.parse(createCall![1].body);
      expect(body.budgetAmount).toBe(500000);

      const defaultsUpdateCall = mockFetch.mock.calls.find(
        (call) =>
          typeof call[0] === "string" &&
          call[0].includes("/api/finance/defaults") &&
          call[1]?.method === "PUT",
      );
      expect(defaultsUpdateCall).toBeUndefined();
    });
  });

  describe("error state", () => {
    it("renders error state on server error", async () => {
      mockServerError("Database connection failed");
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Something went wrong")).toBeInTheDocument();
      });

      expect(
        screen.getByRole("button", { name: /retry/i }),
      ).toBeInTheDocument();
    });

    it("retries fetch on retry button click", async () => {
      mockServerError("Temporary failure");
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Something went wrong")).toBeInTheDocument();
      });

      mockPeriodFound();
      mockDashboardDataEmpty();
      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: /retry/i }));

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });
    });
  });

  describe("recent expenses", () => {
    it("shows recent expenses when expenses exist", async () => {
      mockPeriodFound();
      mockDashboardDataWithExpenses();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Recent Expenses")).toBeInTheDocument();
      });

      expect(screen.getByText("Groceries")).toBeInTheDocument();
      expect(screen.getByText("Coffee")).toBeInTheDocument();
      expect(screen.getByText("$500.00")).toBeInTheDocument();
      expect(screen.getByText("$45.00")).toBeInTheDocument();
    });

    it("shows View All link to /expenses", async () => {
      mockPeriodFound();
      mockDashboardDataWithExpenses();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Recent Expenses")).toBeInTheDocument();
      });

      const viewAllLink = screen.getByRole("link", { name: /view all/i });
      expect(viewAllLink).toHaveAttribute("href", "/expenses");
    });

    it("updates total spent in summary bar from summary endpoint", async () => {
      mockPeriodFound();
      mockDashboardDataWithExpenses();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Recent Expenses")).toBeInTheDocument();
      });

      // totalSpent from summary: 54500 cents = $545.00
      // Appears in both summary bar and historical comparison widget
      await waitFor(() => {
        expect(screen.getAllByText("$545.00").length).toBeGreaterThanOrEqual(1);
      });

      // Remaining: $3,000.00 - $545.00 = $2,455.00
      expect(screen.getByText("$2,455.00")).toBeInTheDocument();
    });
  });

  describe("category gauges", () => {
    it("renders three category gauges with allocated and spent amounts", async () => {
      mockPeriodFound();
      mockDashboardDataWithExpenses();
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
      mockPeriodFound();
      // Override summary to have essentials over budget
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            summary: {
              ...mockSummary,
              essentials: { allocated: 150000, spent: 200000, remaining: -50000, percentUsed: 133.33 },
            },
          }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ tagSpending: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ points: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
      });
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByTestId("gauge-essentials")).toBeInTheDocument();
      });

      const essentialsGauge = screen.getByTestId("gauge-essentials");
      expect(within(essentialsGauge).getByText("133%")).toBeInTheDocument();
      expect(within(essentialsGauge).getByText(/over by/i)).toBeInTheDocument();
    });

    it("shows over-budget indicator for zero-allocation category with spending", async () => {
      mockPeriodFound();
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            summary: {
              ...mockSummary,
              // 0% allocation with $50 spent: percentUsed > 100
              savings: { allocated: 0, spent: 5000, remaining: -5000, percentUsed: 150 },
            },
          }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ tagSpending: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ points: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
      });
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
      mockPeriodFound();
      mockDashboardDataWithExpenses();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Spending Pace")).toBeInTheDocument();
      });

      expect(screen.getByText("Daily Average")).toBeInTheDocument();
      expect(screen.getByText("Required Rate")).toBeInTheDocument();
      expect(screen.getByText("Status")).toBeInTheDocument();
    });

    it("shows on-track indicator when spending is under pace", async () => {
      mockPeriodFound();
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            summary: { ...mockSummary, isOnTrack: true, totalSpent: 10000, remaining: 290000 },
          }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ tagSpending: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ points: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
      });
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("On Track")).toBeInTheDocument();
      });
    });

    it("shows over-budget amount when totalSpent exceeds budget", async () => {
      mockPeriodFound();
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            summary: {
              ...mockSummary,
              totalBudget: 100000,
              totalSpent: 150000,
              remaining: -50000,
              isOnTrack: false,
            },
          }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ tagSpending: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ points: [] }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true, status: 200,
        json: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
      });
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/over by \$500\.00/i)).toBeInTheDocument();
      });
    });
  });

  describe("dashboard data error", () => {
    it("does not crash when dashboard data fetch fails", async () => {
      mockPeriodFound();
      // All 4 parallel fetches fail: toast handles the error, dashboard stays up
      mockFetch.mockRejectedValueOnce(new Error("Network error"));
      mockFetch.mockRejectedValueOnce(new Error("Network error"));
      mockFetch.mockRejectedValueOnce(new Error("Network error"));
      mockFetch.mockRejectedValueOnce(new Error("Network error"));
      renderDashboard();

      // Dashboard header should still render even with data errors
      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });
    });
  });

  describe("responsive layout", () => {
    it("shows Log Expense button on mobile viewport", async () => {
      mockPeriodFound();
      mockDashboardDataEmpty();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      // The mobile Log Expense button has md:hidden class
      const logExpenseLinks = screen.getAllByRole("link", { name: /log expense/i });
      expect(logExpenseLinks.length).toBeGreaterThanOrEqual(1);
    });

    it("charts container has hidden md:block class for mobile hiding", async () => {
      mockPeriodFound();
      mockDashboardDataWithExpenses();
      const { container } = renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Spending by Tag")).toBeInTheDocument();
      });

      // Verify the charts wrapper has the responsive hidden class
      const hiddenCharts = container.querySelector(".hidden.md\\:block");
      expect(hiddenCharts).not.toBeNull();
    });
  });

  describe("budget settings editor", () => {
    it("shows budget settings editor when gear button is clicked", async () => {
      mockPeriodFound();
      mockDashboardDataWithExpenses();
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
      mockPeriodFound();
      mockDashboardDataWithExpenses();
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
      mockPeriodFound();
      mockDashboardDataWithExpenses();
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
      mockPeriodFound();
      mockDashboardDataWithExpenses();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByTestId("historical-comparison")).toBeInTheDocument();
      });

      expect(screen.getByText("Historical Comparison")).toBeInTheDocument();
      expect(screen.getByText("Current Period")).toBeInTheDocument();
      expect(screen.getByText("Previous Period")).toBeInTheDocument();
      expect(screen.getByText("$480.00")).toBeInTheDocument(); // previousSpent
      expect(screen.getByText(/13\.5% from last period/)).toBeInTheDocument();
    });

    it("shows 'not enough data' when only one period exists", async () => {
      mockPeriodFound();
      // Mock dashboard data but comparison returns only-one-period data
      // summary
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ summary: mockSummary }),
      });
      // spending/by-tag
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ tagSpending: mockTagSpending }),
      });
      // spending/cumulative
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ points: mockCumulativeData }),
      });
      // recent expenses
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            data: mockExpenses,
            total: 2,
            page: 1,
            pageSize: 5,
            hasMore: false,
          }),
      });
      // comparison: only one period
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            comparison: {
              currentSpent: 54500,
              previousSpent: 0,
              rollingAverage: null,
              changePercent: 0,
            },
          }),
      });

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
