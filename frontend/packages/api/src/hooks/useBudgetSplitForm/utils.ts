import {
  DEFAULT_BUDGET_SPLIT,
  getMinorUnitDigits,
  toMajorUnits,
} from "@gofin/core";
import type {
  BudgetSplitFields,
  BudgetSplitFormOptions,
} from "./types";

export interface SplitPercentageFields {
  essentials: string;
  desires: string;
  savings: string;
}

export interface SplitPercentages {
  essentials: number;
  desires: number;
  savings: number;
}

/** Blank or non-numeric input falls back to 0. */
export function parseSplitPercentages(
  fields: SplitPercentageFields,
): SplitPercentages {
  return {
    essentials: parseInt(fields.essentials, 10) || 0,
    desires: parseInt(fields.desires, 10) || 0,
    savings: parseInt(fields.savings, 10) || 0,
  };
}

export function precisionError(currency: string): string {
  const minorUnitDigits = getMinorUnitDigits(currency);
  if (minorUnitDigits === 0) {
    return `Budget amount must be a whole ${currency} amount`;
  }
  return `Budget amount supports up to ${minorUnitDigits} decimal places for ${currency}`;
}

export function buildInitialFields(options?: BudgetSplitFormOptions): BudgetSplitFields {
  const split = options?.initialSplit ?? DEFAULT_BUDGET_SPLIT;
  const currency = options?.currency ?? "USD";
  let budgetDollars = "";
  if (options?.initialBudgetCents != null) {
    budgetDollars = toMajorUnits(options.initialBudgetCents, currency).toString();
  }

  return {
    budgetDollars,
    essentials: String(split.essentials),
    desires: String(split.desires),
    savings: String(split.savings),
  };
}
