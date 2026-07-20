import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DashboardFeature } from "../index";
import {
  buildUser,
  buildPeriod,
  buildPeriodSummary,
  createMockApi,
  renderWithRouter,
} from "@gofin/test-utils";

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

function dashboardDataRoutes() {
  return {
    "/api/finance/summary": { body: { summary: testSummary } },
    "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
    "/api/finance/spending/cumulative": { body: { points: [] } },
    "/api/expenses": { body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false } },
    "/api/finance/spending/comparison": {
      status: 404,
      body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
    },
    "/api/finance/prorata/upcoming": { body: { schedules: [] } },
    "/api/finance/spending/trends": { body: { trends: [] } },
  };
}

function renderDashboard(user = testUser) {
  return renderWithRouter(<DashboardFeature user={user} />, { route: "/dashboard" });
}

describe("DashboardFeature - Budget Settings Editor Save", () => {
  beforeEach(() => {
    // Each test sets its own fetch mock
  });

  it("saves budget settings and closes editor on success", async () => {
    const updatedPeriod = buildPeriod({
      ...testPeriod,
      budgetAmount: 400000,
      essentialsPercent: 60,
      desiresPercent: 25,
      savingsPercent: 15,
    });

    const mockApi = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      ...dashboardDataRoutes(),
      [`/api/finance/periods/${testPeriod.id}`]: { body: { period: updatedPeriod } },
    });
    global.fetch = mockApi as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByLabelText("Budget Settings"));

    const budgetInput = screen.getByLabelText("Monthly Budget");
    await user.clear(budgetInput);
    await user.type(budgetInput, "4000");

    const essentialsInput = screen.getByLabelText("Essentials %");
    await user.clear(essentialsInput);
    await user.type(essentialsInput, "60");

    const desiresInput = screen.getByLabelText("Desires %");
    await user.clear(desiresInput);
    await user.type(desiresInput, "25");

    const savingsInput = screen.getByLabelText("Savings %");
    await user.clear(savingsInput);
    await user.type(savingsInput, "15");

    await user.click(screen.getByText("Save Changes"));

    // Editor should close after successful save
    await waitFor(() => {
      expect(screen.queryByTestId("budget-settings-editor")).not.toBeInTheDocument();
    });

    // Verify PUT was called with correct payload
    const putCall = mockApi._calls.find(
      (call) =>
        call.url.includes(`/api/finance/periods/${testPeriod.id}`) &&
        call.method === "PUT",
    );
    expect(putCall).toBeDefined();
    expect((putCall!.body as { budgetAmount: number }).budgetAmount).toBe(400000);
    expect((putCall!.body as { essentialsPercent: number }).essentialsPercent).toBe(60);
    expect((putCall!.body as { desiresPercent: number }).desiresPercent).toBe(25);
    expect((putCall!.body as { savingsPercent: number }).savingsPercent).toBe(15);
  });

  it("shows API error when budget save fails", async () => {
    const mockApi = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      ...dashboardDataRoutes(),
      [`/api/finance/periods/${testPeriod.id}`]: {
        status: 500,
        body: { code: "INTERNAL_SERVER_ERROR", message: "Database error" },
      },
    });
    global.fetch = mockApi as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByLabelText("Budget Settings"));
    await user.click(screen.getByText("Save Changes"));

    await waitFor(() => {
      expect(screen.getByText("Database error")).toBeInTheDocument();
    });

    // Editor should remain open on error
    expect(screen.getByTestId("budget-settings-editor")).toBeInTheDocument();
  });

  it("shows split total indicator in the editor", async () => {
    global.fetch = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      ...dashboardDataRoutes(),
    }) as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByLabelText("Budget Settings"));

    // Should show total split percentage
    expect(screen.getByText(/Total: 100%/)).toBeInTheDocument();
  });

  it("shows upcoming pro-rata schedules section", async () => {
    global.fetch = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      "/api/finance/summary": { body: { summary: testSummary } },
      "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
      "/api/finance/spending/cumulative": { body: { points: [] } },
      "/api/expenses": { body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false } },
      "/api/finance/spending/comparison": {
        status: 404,
        body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
      },
      "/api/finance/prorata/upcoming": {
        body: {
          schedules: [
            {
              id: "prorata-1",
              name: "Annual Insurance",
              amount: 10000,
              installmentIndex: 3,
              installmentTotal: 12,
              expenseType: "essentials",
              expenseDate: "2026-06-01",
            },
          ],
        },
      },
      "/api/finance/spending/trends": { body: { trends: [] } },
    }) as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Upcoming Pro-rata")).toBeInTheDocument();
    });

    expect(screen.getByText("Annual Insurance")).toBeInTheDocument();
    expect(screen.getByText("Installment 3 of 12")).toBeInTheDocument();
  });

  it("shows monthly trends section when trend data exists", async () => {
    const trendData = [
      {
        year: 2026, month: 4, totalSpent: 250000, budgetAmount: 300000,
        essentialsSpent: 125000, desiresSpent: 75000, savingsSpent: 50000,
        essentialsPercent: 50, desiresPercent: 30, savingsPercent: 20,
      },
      {
        year: 2026, month: 5, totalSpent: 54500, budgetAmount: 300000,
        essentialsSpent: 50000, desiresSpent: 4500, savingsSpent: 0,
        essentialsPercent: 50, desiresPercent: 30, savingsPercent: 20,
      },
    ];

    global.fetch = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      "/api/finance/summary": { body: { summary: testSummary } },
      "/api/finance/spending/by-tag": { body: { tagSpending: [] } },
      "/api/finance/spending/cumulative": { body: { points: [] } },
      "/api/expenses": { body: { data: [], total: 0, page: 1, pageSize: 5, hasMore: false } },
      "/api/finance/spending/comparison": {
        status: 404,
        body: { code: "PERIOD_NOT_FOUND", message: "Not enough data" },
      },
      "/api/finance/prorata/upcoming": { body: { schedules: [] } },
      "/api/finance/spending/trends": { body: { trends: trendData } },
    }) as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByLabelText("Select trend chart")).toBeInTheDocument();
    });

    // Default chart is "Monthly Spending" shown in Select trigger and chart title
    expect(screen.getAllByText("Monthly Spending").length).toBeGreaterThanOrEqual(1);
  });

  it("shows network error message when budget save has connection failure", async () => {
    const mockApi = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      ...dashboardDataRoutes(),
    });
    global.fetch = mockApi as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByLabelText("Budget Settings"));

    // Override fetch to simulate network failure
    global.fetch = (() => Promise.reject(new TypeError("Failed to fetch"))) as unknown as typeof fetch;

    await user.click(screen.getByText("Save Changes"));

    await waitFor(() => {
      expect(
        screen.getByText("Connection lost. Check your internet and try again."),
      ).toBeInTheDocument();
    });

    // Editor should remain open on error
    expect(screen.getByTestId("budget-settings-editor")).toBeInTheDocument();
  });
});
