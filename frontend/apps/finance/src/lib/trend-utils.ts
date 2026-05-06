/** Result of computing category spending as percentages of total. */
export interface CategoryPercentages {
  essentials: number;
  desires: number;
  savings: number;
}

/**
 * Compute each category's spending as a percentage of total spent.
 * Returns 0 for all categories when total is zero (no division-by-zero).
 * Rounds to 1 decimal place.
 */
export function computeCategoryPercentages(
  essentialsSpent: number,
  desiresSpent: number,
  savingsSpent: number,
): CategoryPercentages {
  const total = essentialsSpent + desiresSpent + savingsSpent;

  if (total === 0) {
    return { essentials: 0, desires: 0, savings: 0 };
  }

  return {
    essentials: roundToOneDecimal((essentialsSpent / total) * 100),
    desires: roundToOneDecimal((desiresSpent / total) * 100),
    savings: roundToOneDecimal((savingsSpent / total) * 100),
  };
}

function roundToOneDecimal(value: number): number {
  return Math.round(value * 10) / 10;
}
