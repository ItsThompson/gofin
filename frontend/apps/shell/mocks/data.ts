import type { User } from "@gofin/core";

// Finance-specific types for mock data (local definitions to avoid coupling shell to finance app)

interface BudgetPeriod {
  id: string;
  userId: string;
  year: number;
  month: number;
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  createdAt: string;
  updatedAt: string;
}

interface DefaultSettings {
  userId: string;
  budgetAmount: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
  currency: string;
  createdAt: string;
  updatedAt: string;
}

interface Expense {
  id: string;
  userId: string;
  name: string;
  amount: number;
  currency: string;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  expenseDate: string;
  periodYear: number;
  periodMonth: number;
  status: "active" | "corrected";
  isProRata: boolean;
  createdAt: string;
}

interface Tag {
  id: string;
  name: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

interface CategorySummary {
  allocated: number;
  spent: number;
  remaining: number;
  percentUsed: number;
}

interface PeriodSummary {
  periodId: string;
  year: number;
  month: number;
  totalBudget: number;
  totalSpent: number;
  remaining: number;
  daysInPeriod: number;
  daysElapsed: number;
  dailySpendRate: number;
  budgetPace: number;
  isOnTrack: boolean;
  essentials: CategorySummary;
  desires: CategorySummary;
  savings: CategorySummary;
}

interface TagSpending {
  tagId: string;
  tagName: string;
  amount: number;
  percentOfTotal: number;
}

interface CumulativeSpendPoint {
  day: number;
  actual: number;
  ideal: number;
}

interface HistoricalComparison {
  currentSpent: number;
  previousSpent: number;
  rollingAverage: number | null;
  changePercent: number;
}

interface ProRataSchedule {
  id: string;
  userId: string;
  proRataGroup: string;
  name: string;
  amount: number;
  currency: string;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  targetYear: number;
  targetMonth: number;
  installmentIndex: number;
  installmentTotal: number;
  status: "pending" | "applied";
  createdAt: string;
  appliedAt: string | null;
}

interface TrendPoint {
  year: number;
  month: number;
  totalSpent: number;
  budgetAmount: number;
  essentialsSpent: number;
  desiresSpent: number;
  savingsSpent: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
}

// --- Helpers ---

const now = new Date();
const currentYear = now.getFullYear();
const currentMonth = now.getMonth() + 1;
const daysInMonth = new Date(currentYear, currentMonth, 0).getDate();
const daysElapsed = now.getDate();

function uuid(): string {
  return crypto.randomUUID();
}

function daysAgoISO(days: number): string {
  const date = new Date(now);
  date.setDate(date.getDate() - days);
  return date.toISOString().slice(0, 10);
}

// --- Users ---

export const adminUser: User = {
  id: "u-admin-001",
  username: "admin",
  email: "admin@gofin.local",
  role: "admin",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-04-01T00:00:00Z",
};

export const regularUser: User = {
  id: "u-user-001",
  username: "alex",
  email: "alex@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-04-10T00:00:00Z",
};

/** The user that the mock API authenticates as. Change this to test different roles. */
export let currentMockUser: User = adminUser;

export function setCurrentMockUser(user: User): void {
  currentMockUser = user;
}

// --- Tags ---

const tagIds = {
  bills: uuid(),
  food: uuid(),
  household: uuid(),
  investment: uuid(),
  personalCare: uuid(),
  recreation: uuid(),
  selfInvestment: uuid(),
  social: uuid(),
  transport: uuid(),
  travel: uuid(),
  coffee: uuid(),
};

export const mockTags: Tag[] = [
  { id: tagIds.bills, name: "Bills", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.coffee, name: "Coffee", isDefault: false, createdAt: "2026-04-15T00:00:00Z", updatedAt: "2026-04-15T00:00:00Z" },
  { id: tagIds.food, name: "Food", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.household, name: "Household", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.investment, name: "Investment", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.personalCare, name: "Personal Care", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.recreation, name: "Recreation/Entertainment", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.selfInvestment, name: "Self Investment", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.social, name: "Social", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.transport, name: "Transport", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.travel, name: "Travel", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
];

// --- Budget Period ---

export const mockPeriod: BudgetPeriod = {
  id: "bp-001",
  userId: adminUser.id,
  year: currentYear,
  month: currentMonth,
  budgetAmount: 300000, // $3,000
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  createdAt: `${currentYear}-${String(currentMonth).padStart(2, "0")}-01T00:00:00Z`,
  updatedAt: `${currentYear}-${String(currentMonth).padStart(2, "0")}-01T00:00:00Z`,
};

export const mockDefaults: DefaultSettings = {
  userId: adminUser.id,
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
  createdAt: "2026-04-01T00:00:00Z",
  updatedAt: "2026-04-01T00:00:00Z",
};

// --- Expenses ---

export const mockExpenses: Expense[] = [
  {
    id: uuid(), userId: adminUser.id, name: "Rent", amount: 120000,
    currency: "USD", expenseType: "essentials", tagId: tagIds.bills,
    expenseDate: daysAgoISO(0), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Grocery run", amount: 8540,
    currency: "USD", expenseType: "essentials", tagId: tagIds.food,
    expenseDate: daysAgoISO(1), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Concert tickets", amount: 15000,
    currency: "USD", expenseType: "desires", tagId: tagIds.recreation,
    expenseDate: daysAgoISO(2), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Bus pass", amount: 7500,
    currency: "USD", expenseType: "essentials", tagId: tagIds.transport,
    expenseDate: daysAgoISO(3), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Online course", amount: 4999,
    currency: "USD", expenseType: "savings", tagId: tagIds.selfInvestment,
    expenseDate: daysAgoISO(4), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Dinner with friends", amount: 4200,
    currency: "USD", expenseType: "desires", tagId: tagIds.social,
    expenseDate: daysAgoISO(5), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Coffee beans", amount: 1899,
    currency: "USD", expenseType: "desires", tagId: tagIds.coffee,
    expenseDate: daysAgoISO(6), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
];

// --- Period Summary ---

function computeSummary(): PeriodSummary {
  const totalSpent = mockExpenses.reduce((sum, expense) => sum + expense.amount, 0);
  const totalBudget = mockPeriod.budgetAmount;
  const remaining = totalBudget - totalSpent;
  const daysRemaining = daysInMonth - daysElapsed;
  const essentialsAllocated = Math.round(totalBudget * mockPeriod.essentialsPercent / 100);
  const desiresAllocated = Math.round(totalBudget * mockPeriod.desiresPercent / 100);
  const savingsAllocated = totalBudget - essentialsAllocated - desiresAllocated;

  const essentialsSpent = mockExpenses
    .filter((expense) => expense.expenseType === "essentials")
    .reduce((sum, expense) => sum + expense.amount, 0);
  const desiresSpent = mockExpenses
    .filter((expense) => expense.expenseType === "desires")
    .reduce((sum, expense) => sum + expense.amount, 0);
  const savingsSpent = mockExpenses
    .filter((expense) => expense.expenseType === "savings")
    .reduce((sum, expense) => sum + expense.amount, 0);

  const dailySpendRate = daysElapsed > 0 ? Math.round(totalSpent / daysElapsed) : 0;
  const idealDailyRate = daysInMonth > 0 ? Math.round(totalBudget / daysInMonth) : 0;
  const budgetPace = daysRemaining > 0 ? Math.round(remaining / daysRemaining) : 0;

  return {
    periodId: mockPeriod.id,
    year: currentYear,
    month: currentMonth,
    totalBudget,
    totalSpent,
    remaining,
    daysInPeriod: daysInMonth,
    daysElapsed,
    dailySpendRate,
    budgetPace,
    isOnTrack: dailySpendRate <= idealDailyRate,
    essentials: {
      allocated: essentialsAllocated,
      spent: essentialsSpent,
      remaining: essentialsAllocated - essentialsSpent,
      percentUsed: essentialsAllocated > 0 ? (essentialsSpent / essentialsAllocated) * 100 : 0,
    },
    desires: {
      allocated: desiresAllocated,
      spent: desiresSpent,
      remaining: desiresAllocated - desiresSpent,
      percentUsed: desiresAllocated > 0 ? (desiresSpent / desiresAllocated) * 100 : 0,
    },
    savings: {
      allocated: savingsAllocated,
      spent: savingsSpent,
      remaining: savingsAllocated - savingsSpent,
      percentUsed: savingsAllocated > 0 ? (savingsSpent / savingsAllocated) * 100 : 0,
    },
  };
}

export const mockSummary: PeriodSummary = computeSummary();

// --- Tag Spending ---

export function computeTagSpending(): TagSpending[] {
  const totalSpent = mockExpenses.reduce((sum, expense) => sum + expense.amount, 0);
  const byTag = new Map<string, number>();
  mockExpenses.forEach((expense) => {
    byTag.set(expense.tagId, (byTag.get(expense.tagId) ?? 0) + expense.amount);
  });

  return Array.from(byTag.entries())
    .map(([tagId, amount]) => ({
      tagId,
      tagName: mockTags.find((tag) => tag.id === tagId)?.name ?? "Unknown",
      amount,
      percentOfTotal: totalSpent > 0 ? (amount / totalSpent) * 100 : 0,
    }))
    .sort((a, b) => b.amount - a.amount);
}

// --- Cumulative Spend ---

export function computeCumulativeSpend(): CumulativeSpendPoint[] {
  const dailyIdeal = mockPeriod.budgetAmount / daysInMonth;
  const points: CumulativeSpendPoint[] = [];
  let cumulative = 0;

  for (let day = 1; day <= Math.min(daysElapsed, daysInMonth); day++) {
    const dayStr = String(day).padStart(2, "0");
    const monthStr = String(currentMonth).padStart(2, "0");
    const dateStr = `${currentYear}-${monthStr}-${dayStr}`;

    const dayExpenses = mockExpenses
      .filter((expense) => expense.expenseDate === dateStr)
      .reduce((sum, expense) => sum + expense.amount, 0);

    cumulative += dayExpenses;

    points.push({
      day,
      actual: cumulative,
      ideal: Math.round(dailyIdeal * day),
    });
  }

  return points;
}

// --- Historical Comparison ---

export const mockComparison: HistoricalComparison = {
  currentSpent: mockSummary.totalSpent,
  previousSpent: 285000, // $2,850 last month
  rollingAverage: 275000, // $2,750 rolling 3-month avg
  changePercent: -5.3,
};

// --- Upcoming Pro-rata ---

export const mockUpcomingProRata: ProRataSchedule[] = [
  {
    id: uuid(),
    userId: adminUser.id,
    proRataGroup: uuid(),
    name: "New laptop",
    amount: 50000, // $500 installment
    currency: "USD",
    expenseType: "essentials",
    tagId: tagIds.selfInvestment,
    targetYear: currentMonth === 12 ? currentYear + 1 : currentYear,
    targetMonth: currentMonth === 12 ? 1 : currentMonth + 1,
    installmentIndex: 2,
    installmentTotal: 3,
    status: "pending",
    createdAt: "2026-04-01T00:00:00Z",
    appliedAt: null,
  },
];

// --- Admin: All users ---

export const allUsers = [adminUser, regularUser];

// --- Monthly Trends ---

/**
 * Generate mock trend data for N months ending at the current month.
 * Produces realistic varied spending with slight budget variations.
 */
export function computeMockTrends(months: number): TrendPoint[] {
  const points: TrendPoint[] = [];
  let year = currentYear;
  let month = currentMonth;

  // Walk backwards to find the start month, then iterate forward
  for (let i = 0; i < months - 1; i++) {
    month--;
    if (month === 0) {
      month = 12;
      year--;
    }
  }

  for (let i = 0; i < months; i++) {
    // Vary budget slightly month-to-month
    const budgetBase = 280000 + Math.round(Math.sin(i * 0.7) * 20000);
    // Randomize spending around 70-95% of budget
    const spendRatio = 0.7 + (Math.sin(i * 1.3 + 2) + 1) * 0.125;
    const totalSpent = Math.round(budgetBase * spendRatio);

    // Distribute spending across categories with some variance
    const essentialsPct = 45 + Math.round(Math.sin(i * 0.9) * 8);
    const desiresPct = 30 + Math.round(Math.cos(i * 1.1) * 5);

    const essentialsSpent = Math.round(totalSpent * essentialsPct / 100);
    const desiresSpent = Math.round(totalSpent * desiresPct / 100);
    const savingsSpent = totalSpent - essentialsSpent - desiresSpent;

    points.push({
      year,
      month,
      totalSpent,
      budgetAmount: budgetBase,
      essentialsSpent,
      desiresSpent,
      savingsSpent,
      essentialsPercent: 50,
      desiresPercent: 30,
      savingsPercent: 20,
    });

    month++;
    if (month > 12) {
      month = 1;
      year++;
    }
  }

  return points;
}
