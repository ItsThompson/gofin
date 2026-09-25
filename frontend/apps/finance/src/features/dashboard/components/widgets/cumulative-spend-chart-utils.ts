import { formatCurrency, getMinorUnitDigits } from "@gofin/core";

/**
 * Tooltip value formatter for the cumulative spend chart.
 * Returns null for range tuples and invalid scalar values.
 */
export function tooltipFormatter(
  value: unknown,
  name: string,
  currency: string,
): [string, string] | null {
  if (Array.isArray(value)) return null;
  if (typeof value !== "number" && typeof value !== "string") return null;

  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) return null;

  const minorUnitDigits = getMinorUnitDigits(currency);
  const amountMinorUnits = Math.round(numericValue * 10 ** minorUnitDigits);
  return [formatCurrency(amountMinorUnits, currency), name];
}

/**
 * Tooltip label formatter that shows "Day N" for integer days
 * and suppresses display for fractional crossover points.
 */
export function tooltipLabelFormatter(label: unknown): string {
  return Number.isInteger(label) ? `Day ${label}` : "";
}
