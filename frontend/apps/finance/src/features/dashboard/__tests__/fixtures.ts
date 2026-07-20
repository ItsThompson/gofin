import {
  buildUser,
  buildPeriod,
  buildPeriodSummary,
  buildDefaults,
  buildExpense,
} from "@gofin/test-utils";

// --- Shared test data built from factories ---

export const testUser = buildUser({
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  currency: "USD",
});

export const testPeriod = buildPeriod({
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

export const testDefaults = buildDefaults({
  userId: "user-1",
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
});

export const testSummary = buildPeriodSummary({
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

export const testTagSpending = [
  { tagId: "tag-food", tagName: "Food", amount: 50000, percentOfTotal: 91.74 },
  { tagId: "tag-social", tagName: "Social", amount: 4500, percentOfTotal: 8.26 },
];

export const testCumulativeData = Array.from({ length: 31 }, (_, index) => ({
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

export const testExpenses = [
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
export function dashboardDataEmptyRoutes() {
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
export function dashboardDataWithExpensesRoutes() {
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
