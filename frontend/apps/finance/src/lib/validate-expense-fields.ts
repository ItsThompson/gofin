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
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
}

/** Options for validation behavior. */
export interface ValidateExpenseOptions {
  /** Whether to validate pro-rata fields. Default: false. */
  isProRata?: boolean;
  /** Number of pro-rata months (only validated when isProRata is true). */
  proRataMonths?: string;
}

/**
 * Pure validation for expense fields.
 * Returns empty record if all fields are valid.
 * Returns record mapping field names to error messages for invalid fields.
 */
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
  }

  if (!fields.expenseDate) {
    errors.expenseDate = "Date is required";
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
