/**
 * Returns a Tailwind text color class based on remaining budget percentage.
 * - Green (> 30% remaining)
 * - Yellow (10-30% remaining)
 * - Red (< 10% remaining)
 *
 * When budget is $0, remaining is always 100% of nothing: returns empty string.
 */
export function getRemainingColor(budget: number, remaining: number): string {
  if (budget === 0) return "";

  const percent = (remaining / budget) * 100;

  if (percent > 30) return "text-green-600 dark:text-green-400";
  if (percent >= 10) return "text-yellow-600 dark:text-yellow-400";
  return "text-red-600 dark:text-red-400";
}
