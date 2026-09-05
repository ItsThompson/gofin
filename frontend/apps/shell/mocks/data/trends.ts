import type { TrendPoint } from "@gofin/core";
import { currentYear, currentMonth } from "./foundation";

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
      reportingCurrencyCode: "USD",
    });

    month++;
    if (month > 12) {
      month = 1;
      year++;
    }
  }

  return points;
}
