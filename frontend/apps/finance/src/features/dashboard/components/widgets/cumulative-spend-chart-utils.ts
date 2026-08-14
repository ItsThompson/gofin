import { formatCurrency, toMinorUnits } from "@gofin/core";

/**
 * Tooltip value formatter for the cumulative spend chart.
 * Returns null for array values (range area tuples) to suppress them in tooltip.
 */
export function tooltipFormatter(
  value: unknown,
  name: string,
  currency: string,
): [string, string] | null {
  if (Array.isArray(value)) return null;
  return [formatCurrency(toMinorUnits(String(value), currency), currency), name];
}

/**
 * Tooltip label formatter that shows "Day N" for integer days
 * and suppresses display for fractional crossover points.
 */
export function tooltipLabelFormatter(label: unknown): string {
  return Number.isInteger(label) ? `Day ${label}` : "";
}
