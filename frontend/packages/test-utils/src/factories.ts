import type {
  User,
  BudgetPeriod,
  Expense,
  Tag,
  CategorySummary,
  PeriodSummary,
  DefaultSettings,
  ProRataSchedule,
} from "@gofin/core";

const FIXED_DATE = "2026-01-15T00:00:00Z";

const counters: Record<string, number> = {};

function nextId(prefix: string): string {
  counters[prefix] = (counters[prefix] ?? 0) + 1;
  return `${prefix}-${counters[prefix]}`;
}

export function buildUser(overrides?: Partial<User>): User {
  return {
    id: nextId("user"),
    username: "testuser",
    email: "test@example.com",
    role: "user",
    currency: "USD",
    hasCompletedOnboarding: true,
    createdAt: FIXED_DATE,
    ...overrides,
  };
}

export function buildPeriod(overrides?: Partial<BudgetPeriod>): BudgetPeriod {
  return {
    id: nextId("period"),
    userId: "user-1",
    year: 2026,
    month: 1,
    budgetAmount: 300000,
    reportingCurrency: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: FIXED_DATE,
    updatedAt: FIXED_DATE,
    ...overrides,
  };
}

export function buildExpense(overrides?: Partial<Expense>): Expense {
  return {
    id: nextId("expense"),
    userId: "user-1",
    name: "Test Expense",
    transactionCurrency: "USD",
    transactionAmount: 5000,
    reportingAmount: 5000,
    reportingCurrency: "USD",
    expenseType: "essentials",
    tagId: "tag-1",
    expenseDate: "2026-01-15",
    periodYear: 2026,
    periodMonth: 1,
    status: "active",
    isProRata: false,
    createdAt: FIXED_DATE,
    ...overrides,
  };
}

export function buildTag(overrides?: Partial<Tag>): Tag {
  return {
    id: nextId("tag"),
    name: "Food",
    isDefault: false,
    createdAt: FIXED_DATE,
    updatedAt: FIXED_DATE,
    ...overrides,
  };
}

function buildCategorySummary(
  totalBudget: number,
  percent: number,
  spent: number,
): CategorySummary {
  const allocated = Math.round((totalBudget * percent) / 100);
  return {
    allocated,
    spent,
    remaining: allocated - spent,
    percentUsed: allocated > 0 ? Math.round((spent / allocated) * 10000) / 100 : 0,
  };
}

export function buildPeriodSummary(overrides?: Partial<PeriodSummary>): PeriodSummary {
  const totalBudget = overrides?.totalBudget ?? 300000;
  const essentialsPercent = 50;
  const desiresPercent = 30;
  const savingsPercent = 20;

  const essentials = overrides?.essentials ?? buildCategorySummary(totalBudget, essentialsPercent, 0);
  const desires = overrides?.desires ?? buildCategorySummary(totalBudget, desiresPercent, 0);
  const savings = overrides?.savings ?? buildCategorySummary(totalBudget, savingsPercent, 0);

  const totalSpent = overrides?.totalSpent ?? 0;
  const remaining = overrides?.remaining ?? totalBudget - totalSpent;
  const daysInPeriod = overrides?.daysInPeriod ?? 31;
  const daysElapsed = overrides?.daysElapsed ?? 15;
  const dailySpendRate = overrides?.dailySpendRate ?? (daysElapsed > 0 ? Math.round(totalSpent / daysElapsed) : 0);
  const daysRemaining = daysInPeriod - daysElapsed;
  const budgetPace = overrides?.budgetPace ?? (daysRemaining > 0 ? Math.round(remaining / daysRemaining) : 0);

  return {
    periodId: overrides?.periodId ?? "period-1",
    year: overrides?.year ?? 2026,
    month: overrides?.month ?? 1,
    totalBudget,
    totalSpent,
    remaining,
    daysInPeriod,
    daysElapsed,
    dailySpendRate,
    budgetPace,
    isOnTrack: overrides?.isOnTrack ?? true,
    essentials,
    desires,
    savings,
  };
}

export function buildDefaults(overrides?: Partial<DefaultSettings>): DefaultSettings {
  return {
    userId: "user-1",
    budgetAmount: 300000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    currency: "USD",
    createdAt: FIXED_DATE,
    updatedAt: FIXED_DATE,
    ...overrides,
  };
}

export function buildProRataSchedule(overrides?: Partial<ProRataSchedule>): ProRataSchedule {
  return {
    id: nextId("prorata"),
    userId: "user-1",
    proRataGroup: "group-1",
    name: "Test Pro-Rata",
    amount: 10000,
    transactionCurrency: "USD",
    expenseType: "essentials",
    tagId: "tag-1",
    targetYear: 2026,
    targetMonth: 2,
    installmentIndex: 1,
    installmentTotal: 3,
    status: "pending",
    createdAt: FIXED_DATE,
    appliedAt: null,
    ...overrides,
  };
}
