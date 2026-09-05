import { validateInputPrecision } from "@gofin/core";
import type { ExpenseType } from "@gofin/core";

/** The core expense field values managed by useExpenseFields. */
export interface ExpenseFields {
  /** Expense description/name. */
  name: string;
  /** Amount in dollars as a string (for controlled input binding). */
  amountDollars: string;
  /** Category: essentials, desires, or savings. */
  expenseType: ExpenseType;
  /** Selected tag ID. */
  tagId: string;
  expenseDateIso: string;
}

/** Options for validation behavior. */
export interface ValidateExpenseOptions {
  /** Whether to validate pro-rata fields. Default: false. */
  isProRata?: boolean;
  /** Number of pro-rata months (only validated when isProRata is true). */
  proRataMonths?: string;
  /** Currency used to validate the amount's allowed precision. */
  currency?: string;
}

/**
 * Pure validation for expense fields.
 * Returns empty record if all fields are valid.
 * Returns record mapping field names to error messages for invalid fields.
 */
function isDecimalInput(value: string): boolean {
  return /^([+-]?)(?:(\d+)(\.(\d*)?)?|\.(\d+))$/.test(value.trim());
}

export function validateExpenseFields(
  fields: ExpenseFields,
  options?: ValidateExpenseOptions,
): Record<string, string> {
  const errors: Record<string, string> = {};

  if (!fields.name.trim()) {
    errors.name = "Name is required";
  }

  const parsedAmount = parseFloat(fields.amountDollars);
  if (!fields.amountDollars || isNaN(parsedAmount) || parsedAmount <= 0) {
    errors.amount = "Amount must be greater than 0";
  } else if (!isDecimalInput(fields.amountDollars)) {
    errors.amount = "Enter a valid amount";
  } else if (options?.currency) {
    const precisionValidation = validateInputPrecision(
      fields.amountDollars,
      options.currency,
    );
    if (!precisionValidation.isValid) {
      errors.amount = precisionValidation.fieldError ?? "Invalid amount precision";
    }
  }

  if (!fields.expenseDateIso) {
    errors.expenseDateIso = "Date is required";
  }

  if (!fields.tagId) {
    errors.tagId = "Tag is required";
  }

  if (options?.isProRata) {
    const months = parseInt(options.proRataMonths ?? "", 10);
    if (!options.proRataMonths || isNaN(months) || months < 2) {
      errors.proRataMonths = "Must be at least 2 months";
    }
  }

  return errors;
}
