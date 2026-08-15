import {
  getCurrencyDefinition,
  getCurrencySymbol,
  getMinorUnitDigits,
} from "./currencyCatalog";

export interface CurrencyValidationResult {
  isValid: boolean;
  fieldError?: string;
}

export function getCurrencyInputStep(currencyCode: string): string {
  const minorUnitDigits = getMinorUnitDigits(currencyCode);
  if (minorUnitDigits === 0) return "1";
  return `0.${"0".repeat(minorUnitDigits - 1)}1`;
}

export function validateInputPrecision(
  amountString: string,
  currencyCode: string,
): CurrencyValidationResult {
  const trimmed = amountString.trim();
  if (trimmed === "") return { isValid: true };
  if (!Number.isFinite(Number(trimmed))) return { isValid: true };

  const minorUnitDigits = getMinorUnitDigits(currencyCode);
  const [, fraction = ""] = trimmed.split(".");
  if (fraction.length <= minorUnitDigits) return { isValid: true };

  const code = getCurrencyDefinition(currencyCode)?.code ?? currencyCode;
  const fieldError = minorUnitDigits === 0
    ? `Amount must be a whole ${code} amount`
    : `Amount supports up to ${minorUnitDigits} decimal places for ${code}`;

  return { isValid: false, fieldError };
}

export function hasValidMinorUnitPrecision(
  amountString: string,
  currencyCode: string,
): boolean {
  return validateInputPrecision(amountString, currencyCode).isValid;
}

export function toMajorUnits(
  amountMinorUnits: number,
  currencyCode: string,
): number {
  return amountMinorUnits / 10 ** getMinorUnitDigits(currencyCode);
}

export function formatAmount(
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

export function formatCurrency(
  amountMinorUnits: number,
  currencyCode: string,
): string {
  return formatAmount(amountMinorUnits, currencyCode);
}

export function parseInput(
  amountString: string,
  currencyCode: string,
): number {
  const trimmed = amountString.trim();
  if (trimmed === "") return 0;

  const match = /^([+-]?)(?:(\d+)(?:\.(\d*))?|\.(\d+))$/.exec(trimmed);
  if (!match) return 0;

  const minorUnitDigits = getMinorUnitDigits(currencyCode);
  const [, sign, whole = "0", fractionWithWhole = "", fractionOnly = ""] = match;
  const fraction = fractionOnly || fractionWithWhole;
  if (fraction.length > minorUnitDigits) {
    throw new Error(
      validateInputPrecision(amountString, currencyCode).fieldError ??
        `Amount supports up to ${minorUnitDigits} decimal places for ${currencyCode}`,
    );
  }

  const paddedFraction = fraction.padEnd(minorUnitDigits, "0");
  const absoluteMinorUnits = Number(whole) * 10 ** minorUnitDigits +
    Number(paddedFraction || "0");

  return sign === "-" ? -absoluteMinorUnits : absoluteMinorUnits;
}

export function toMinorUnits(
  amountString: string,
  currencyCode: string,
): number {
  return parseInput(amountString, currencyCode);
}

export function toCents(dollarString: string): number {
  return parseInput(dollarString, "USD");
}
