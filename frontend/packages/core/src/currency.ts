import { getCurrencySymbol, getMinorUnitDigits } from "./currencyCatalog";

export function getCurrencyInputStep(currencyCode: string): string {
  const minorUnitDigits = getMinorUnitDigits(currencyCode);
  if (minorUnitDigits === 0) return "1";
  return `0.${"0".repeat(minorUnitDigits - 1)}1`;
}

export function hasValidMinorUnitPrecision(
  amountString: string,
  currencyCode: string,
): boolean {
  const trimmed = amountString.trim();
  if (trimmed === "") return true;
  if (!Number.isFinite(Number(trimmed))) return true;

  const [, fraction = ""] = trimmed.split(".");
  return fraction.length <= getMinorUnitDigits(currencyCode);
}

export function toMajorUnits(
  amountMinorUnits: number,
  currencyCode: string,
): number {
  return amountMinorUnits / 10 ** getMinorUnitDigits(currencyCode);
}

export function formatCurrency(
  amountMinorUnits: number,
  currencyCode: string,
): string {
  const symbol = getCurrencySymbol(currencyCode);
  const minorUnitDigits = getMinorUnitDigits(currencyCode);
  const majorUnits = Math.abs(toMajorUnits(amountMinorUnits, currencyCode));
  const formatted = majorUnits.toLocaleString("en-US", {
    minimumFractionDigits: minorUnitDigits,
    maximumFractionDigits: minorUnitDigits,
  });
  const prefix = amountMinorUnits < 0 ? "-" : "";
  return `${prefix}${symbol}${formatted}`;
}

export function toMinorUnits(
  amountString: string,
  currencyCode: string,
): number {
  const parsed = parseFloat(amountString);
  if (Number.isNaN(parsed)) return 0;
  return Math.round(parsed * 10 ** getMinorUnitDigits(currencyCode));
}

export function toCents(dollarString: string): number {
  return toMinorUnits(dollarString, "USD");
}
