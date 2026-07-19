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

interface HealthComponent {
  key: string;
  score: number;
  max: number;
  detail: string;
}

interface HealthInsight {
  summary: string;
  driver: string;
  nudge: string;
}

interface HealthScore {
  year: number;
  month: number;
  total: number;
  band: string;
  provisional: boolean;
  formulaVersion: number;
  components: HealthComponent[];
  insight: HealthInsight;
  configureBudget?: boolean;
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

// --- Financial Health Score ---

// Mirrors the Finance Service compute so the mock stays internally consistent
// with mockPeriod and mockExpenses (dev-mock has no backend). Amounts are cents.

function formatHealthMoney(cents: number): string {
  const negative = cents < 0;
  const abs = Math.abs(cents);
  const dollars = Math.floor(abs / 100);
  const remainder = abs % 100;
  let out = "$" + dollars.toLocaleString("en-US");
  if (remainder !== 0) out += "." + String(remainder).padStart(2, "0");
  return negative ? "-" + out : out;
}

function clampRound(value: number, max: number): number {
  return Math.min(max, Math.max(0, Math.round(value)));
}

// Phase 2 v2 weights and stability tuning: mirror the authoritative Go formula
// in services/finance/internal/service/healthscore.go. dev-mock has no backend,
// so this port must stay faithful to keep the card representative.
const HEALTH_BASE_WEIGHTS = { savings: 25, budget: 25, allocation: 30, stability: 20 };
const STABILITY_MIN_MONTHS = 3;
const STABILITY_COV_CAP = 1.0;

// Synthetic recent closed-month desires totals (cents) so the stability
// sub-score computes in dev-mock. Real values come from the backend.
const mockDesiresWindow = [21000, 19200, 22500, 18700, 20100];

interface HealthWeights {
  savings: number;
  budget: number;
  allocation: number;
  stability: number;
  maxSavings: number;
  maxBudget: number;
  maxAllocation: number;
  maxStability: number;
}

// resolveMockWeights renormalizes the base weights over the present set by
// division, rounding the remainder into allocation, exactly like resolveWeights
// on the backend.
function resolveMockWeights(savingsPresent: boolean, stabilityPresent: boolean): HealthWeights {
  const base = HEALTH_BASE_WEIGHTS;
  let denom = base.budget + base.allocation;
  if (savingsPresent) denom += base.savings;
  if (stabilityPresent) denom += base.stability;
  const eff = (weight: number) => (100 * weight) / denom;

  const savings = savingsPresent ? eff(base.savings) : 0;
  const budget = eff(base.budget);
  const allocation = eff(base.allocation);
  const stability = stabilityPresent ? eff(base.stability) : 0;

  const maxSavings = savingsPresent ? Math.round(savings) : 0;
  const maxBudget = Math.round(budget);
  const maxStability = stabilityPresent ? Math.round(stability) : 0;
  const maxAllocation = 100 - (maxSavings + maxBudget + maxStability);

  return { savings, budget, allocation, stability, maxSavings, maxBudget, maxAllocation, maxStability };
}

// mockStability ports stabilityComponent: full marks for a zero mean, otherwise
// weight * clamp(1 - CoV / cap, 0, 1) with a sample (n-1) standard deviation.
function mockStability(window: number[], weight: number): { score: number; detail: string } {
  const mean = window.reduce((sum, value) => sum + value, 0) / window.length;
  if (mean === 0) return { score: weight, detail: "Desires spend held steady month to month" };

  const variance =
    window.reduce((sum, value) => sum + (value - mean) ** 2, 0) / (window.length - 1);
  const cov = Math.sqrt(variance) / mean;
  const ratio = Math.min(1, Math.max(0, 1 - cov / STABILITY_COV_CAP));
  const pct = Math.round(cov * 100);
  const detail =
    pct <= 0
      ? "Desires spend held steady month to month"
      : `Desires spend varied ~${pct}% month to month`;
  return { score: weight * ratio, detail };
}

export function computeMockHealthScore(): HealthScore {
  const budget = mockPeriod.budgetAmount;
  const savingsTarget = Math.floor((budget * mockPeriod.savingsPercent) / 100);
  const combinedTarget = Math.floor(
    (budget * (mockPeriod.essentialsPercent + mockPeriod.desiresPercent)) / 100,
  );

  const sumByType = (type: string) =>
    mockExpenses
      .filter((expense) => expense.expenseType === type)
      .reduce((sum, expense) => sum + expense.amount, 0);
  const essentialsActual = sumByType("essentials");
  const desiresActual = sumByType("desires");
  const savingsActual = sumByType("savings");
  const edActual = essentialsActual + desiresActual;

  const savingsDropped = savingsTarget === 0;
  const stabilityPresent = mockDesiresWindow.length >= STABILITY_MIN_MONTHS;
  const w = resolveMockWeights(!savingsDropped, stabilityPresent);

  const components: HealthComponent[] = [];

  if (!savingsDropped) {
    const ratio = Math.min(1, savingsActual / savingsTarget);
    components.push({
      key: "savings_achievement",
      score: clampRound(w.savings * ratio, w.maxSavings),
      max: w.maxSavings,
      detail: `Saved ${formatHealthMoney(savingsActual)} of ${formatHealthMoney(savingsTarget)} target`,
    });
  }

  const budgetRatio = combinedTarget === 0 ? (edActual === 0 ? 0 : 2) : edActual / combinedTarget;
  const budgetFactor =
    budgetRatio <= 1 ? 1 : budgetRatio >= 1.5 ? 0 : (1.5 - budgetRatio) / 0.5;
  components.push({
    key: "budget_adherence",
    score: clampRound(w.budget * budgetFactor, w.maxBudget),
    max: w.maxBudget,
    detail: `Spent ${formatHealthMoney(edActual)} of ${formatHealthMoney(combinedTarget)} plan`,
  });

  const spendDenom = savingsDropped ? edActual : edActual + savingsActual;
  const percentSum = savingsDropped
    ? mockPeriod.essentialsPercent + mockPeriod.desiresPercent
    : 100;
  let allocScore = w.allocation;
  let allocDetail = "Balanced across categories";
  const devs: { label: string; dev: number }[] = [];
  if (spendDenom > 0) {
    const devE = essentialsActual / spendDenom - mockPeriod.essentialsPercent / percentSum;
    const devD = desiresActual / spendDenom - mockPeriod.desiresPercent / percentSum;
    let wdev = 0.5 * Math.abs(devE) + (devD > 0 ? 1 : 0.5) * Math.abs(devD);
    devs.push({ label: "Essentials", dev: devE }, { label: "Desires", dev: devD });
    if (!savingsDropped) {
      const devS = savingsActual / spendDenom - mockPeriod.savingsPercent / percentSum;
      wdev += (devS < 0 ? 1 : 0.5) * Math.abs(devS);
      devs.push({ label: "Savings", dev: devS });
    }
    allocScore = w.allocation * (1 - Math.min(1, wdev));
    const over = devs.filter((d) => d.dev > 0).sort((a, b) => b.dev - a.dev)[0];
    if (over && Math.round(over.dev * 100) > 0) {
      allocDetail = `${over.label} ${Math.round(over.dev * 100)} pts over target share`;
    }
  }
  components.push({
    key: "allocation_balance",
    score: clampRound(allocScore, w.maxAllocation),
    max: w.maxAllocation,
    detail: allocDetail,
  });

  if (stabilityPresent) {
    const stability = mockStability(mockDesiresWindow, w.stability);
    components.push({
      key: "spending_stability",
      score: clampRound(stability.score, w.maxStability),
      max: w.maxStability,
      detail: stability.detail,
    });
  }

  const total = components.reduce((sum, component) => sum + component.score, 0);
  const band = total >= 80 ? "green" : total >= 55 ? "amber" : "red";

  const driver = components.reduce((lowest, component) =>
    component.score < lowest.score ? component : lowest,
  );
  const insight = buildMockInsight(driver, {
    savingsGap: savingsTarget - savingsActual,
    overspend: edActual - combinedTarget,
    devs,
    maxSavings: w.maxSavings,
    maxBudget: w.maxBudget,
    maxAllocation: w.maxAllocation,
    maxStability: w.maxStability,
  });

  return {
    year: currentYear,
    month: currentMonth,
    total,
    band,
    provisional: true,
    formulaVersion: 2,
    components,
    insight,
  };
}

function buildMockInsight(
  driver: HealthComponent,
  ctx: {
    savingsGap: number;
    overspend: number;
    devs: { label: string; dev: number }[];
    maxSavings: number;
    maxBudget: number;
    maxAllocation: number;
    maxStability: number;
  },
): HealthInsight {
  if (driver.key === "savings_achievement") {
    return {
      summary: "Savings is the softest score this month.",
      driver: driver.key,
      nudge: `Move an extra ${formatHealthMoney(ctx.savingsGap)} to savings to reach your target and lift your score about ${ctx.maxSavings - driver.score} points.`,
    };
  }
  if (driver.key === "budget_adherence") {
    return {
      summary: "Budget adherence is the softest score this month.",
      driver: driver.key,
      nudge:
        ctx.overspend > 0
          ? `Trim about ${formatHealthMoney(ctx.overspend)} from essentials and desires to get back to plan and lift your score about ${ctx.maxBudget - driver.score} points.`
          : "Keep essentials and desires within your plan to lift this score.",
    };
  }
  if (driver.key === "spending_stability") {
    return {
      summary: "Spending stability is the softest score this month.",
      driver: driver.key,
      nudge: `Steadier discretionary spending month to month could lift your score about ${ctx.maxStability - driver.score} points.`,
    };
  }
  const over = ctx.devs.filter((d) => d.dev > 0).sort((a, b) => b.dev - a.dev)[0];
  const under = ctx.devs.filter((d) => d.dev < 0).sort((a, b) => a.dev - b.dev)[0];
  return {
    summary: "Your category balance is the softest score this month.",
    driver: driver.key,
    nudge:
      over && under
        ? `${over.label} is running ${Math.round(over.dev * 100)} pts over its target share. Shifting spend toward ${under.label} could recover up to ${ctx.maxAllocation - driver.score} points.`
        : `Rebalancing your categories could recover up to ${ctx.maxAllocation - driver.score} points.`,
  };
}

export const mockHealthScore: HealthScore = computeMockHealthScore();

interface HealthScoreTrendPoint {
  year: number;
  month: number;
  total: number;
  band: string;
  provisional: boolean;
  formulaVersion: number;
}

// Sample historical totals for the sparkline (oldest first). The current month
// uses the real computed total and is flagged provisional as the last point.
const MOCK_TREND_TOTALS = [55, 61, 58, 66, 72, 69, 74, 70, 63, 68, 71];

export function computeMockHealthScoreTrend(months: number): HealthScoreTrendPoint[] {
  // Match the Go service clamp policy (AC4): default 6, cap 12.
  const requested = Number.isFinite(months) ? months : 6;
  const count = requested < 1 ? 6 : Math.min(requested, 12);
  const bandFor = (total: number) => (total >= 80 ? "green" : total >= 55 ? "amber" : "red");

  const points: HealthScoreTrendPoint[] = [];
  for (let offset = count - 1; offset >= 0; offset--) {
    let year = currentYear;
    let month = currentMonth - offset;
    while (month <= 0) {
      month += 12;
      year -= 1;
    }
    const provisional = offset === 0;
    const total = provisional
      ? mockHealthScore.total
      : MOCK_TREND_TOTALS[(count - 1 - offset) % MOCK_TREND_TOTALS.length];
    points.push({ year, month, total, band: bandFor(total), provisional, formulaVersion: 2 });
  }
  return points;
}

export const mockHealthScoreTrend: HealthScoreTrendPoint[] = computeMockHealthScoreTrend(6);

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
