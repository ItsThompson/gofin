import type {
  PeriodSummary,
  TagSpending,
  CumulativeSpendPoint,
  HistoricalComparison,
} from "@gofin/core";
import { daysInMonth, daysElapsed, currentYear, currentMonth } from "./foundation";
import { mockPeriod } from "./periods";
import { mockExpenses } from "./expenses";
import { mockTags } from "./tags";

function computeSummary(): PeriodSummary {
  const totalSpent = mockExpenses.reduce((sum, expense) => sum + expense.originalTransactionAmountInMinorUnits, 0);
  const totalBudget = mockPeriod.budgetAmount;
  const remaining = totalBudget - totalSpent;
  const daysRemaining = daysInMonth - daysElapsed;
  const essentialsAllocated = Math.round(totalBudget * mockPeriod.essentialsPercent / 100);
  const desiresAllocated = Math.round(totalBudget * mockPeriod.desiresPercent / 100);
  const savingsAllocated = totalBudget - essentialsAllocated - desiresAllocated;

  const essentialsSpent = mockExpenses
    .filter((expense) => expense.expenseType === "essentials")
    .reduce((sum, expense) => sum + expense.originalTransactionAmountInMinorUnits, 0);
  const desiresSpent = mockExpenses
    .filter((expense) => expense.expenseType === "desires")
    .reduce((sum, expense) => sum + expense.originalTransactionAmountInMinorUnits, 0);
  const savingsSpent = mockExpenses
    .filter((expense) => expense.expenseType === "savings")
    .reduce((sum, expense) => sum + expense.originalTransactionAmountInMinorUnits, 0);

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

export function computeTagSpending(): TagSpending[] {
  const totalSpent = mockExpenses.reduce((sum, expense) => sum + expense.originalTransactionAmountInMinorUnits, 0);
  const byTag = new Map<string, number>();
  mockExpenses.forEach((expense) => {
    byTag.set(expense.tagId, (byTag.get(expense.tagId) ?? 0) + expense.originalTransactionAmountInMinorUnits);
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

export function computeCumulativeSpend(): CumulativeSpendPoint[] {
  const dailyIdeal = mockPeriod.budgetAmount / daysInMonth;
  const points: CumulativeSpendPoint[] = [];
  let cumulative = 0;

  for (let day = 1; day <= Math.min(daysElapsed, daysInMonth); day++) {
    const dayStr = String(day).padStart(2, "0");
    const monthStr = String(currentMonth).padStart(2, "0");
    const dateStr = `${currentYear}-${monthStr}-${dayStr}`;

    const dayExpenses = mockExpenses
      .filter((expense) => expense.expenseDateIso === dateStr)
      .reduce((sum, expense) => sum + expense.originalTransactionAmountInMinorUnits, 0);

    cumulative += dayExpenses;

    points.push({
      day,
      actual: cumulative,
      ideal: Math.round(dailyIdeal * day),
    });
  }

  return points;
}

export const mockComparison: HistoricalComparison = {
  currentSpent: mockSummary.totalSpent,
  previousSpent: 285000, // $2,850 last month
  previousReportingCurrency: "USD",
  comparable: true,
  rollingAverage: 275000, // $2,750 rolling 3-month avg
  changePercent: -5.3,
};
