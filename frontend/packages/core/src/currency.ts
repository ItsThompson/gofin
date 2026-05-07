import { SUPPORTED_CURRENCIES } from "./constants";

/** Map of currency codes to their symbols (derived from SUPPORTED_CURRENCIES). */
const CURRENCY_SYMBOL_MAP = Object.fromEntries(
  SUPPORTED_CURRENCIES.map((currency) => [currency.code, currency.symbol]),
) as Record<string, string>;

/**
 * Get the display symbol for a currency code.
 * Falls back to the code itself if no symbol is mapped.
 */
export function getCurrencySymbol(currencyCode: string): string {
  return CURRENCY_SYMBOL_MAP[currencyCode] ?? currencyCode;
}

/**
 * Format a minor-unit amount (cents) for display.
 * Divides by 100 and prepends the currency symbol.
 */
export function formatCurrency(
  amountCents: number,
  currencyCode: string,
): string {
  const symbol = getCurrencySymbol(currencyCode);
  const majorUnits = Math.abs(amountCents) / 100;
  const formatted = majorUnits.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  const prefix = amountCents < 0 ? "-" : "";
  return `${prefix}${symbol}${formatted}`;
}

/**
 * Convert a dollar string to cents (minor units).
 * Returns 0 for invalid input.
 */
export function toCents(dollarString: string): number {
  const parsed = parseFloat(dollarString);
  if (isNaN(parsed)) return 0;
  return Math.round(parsed * 100);
}
