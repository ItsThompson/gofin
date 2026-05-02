/** Map of currency codes to their symbols. */
const CURRENCY_SYMBOLS: Record<string, string> = {
  USD: "$",
  EUR: "€",
  GBP: "£",
  JPY: "¥",
  CAD: "C$",
  AUD: "A$",
  CHF: "CHF",
  CNY: "¥",
  SGD: "S$",
  HKD: "HK$",
};

/**
 * Get the display symbol for a currency code.
 * Falls back to the code itself if no symbol is mapped.
 */
export function getCurrencySymbol(currencyCode: string): string {
  return CURRENCY_SYMBOLS[currencyCode] ?? currencyCode;
}

/**
 * Format a minor-unit amount (cents) for display.
 * Divides by 100 and prepends the currency symbol.
 *
 * For zero-decimal currencies like JPY, the caller should handle
 * differently (not applicable to the current supported set, where
 * JPY is displayed as ¥X.XX for consistency).
 */
export function formatCurrency(amountCents: number, currencyCode: string): string {
  const symbol = getCurrencySymbol(currencyCode);
  const majorUnits = Math.abs(amountCents) / 100;
  const formatted = majorUnits.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  const prefix = amountCents < 0 ? "-" : "";
  return `${prefix}${symbol}${formatted}`;
}
