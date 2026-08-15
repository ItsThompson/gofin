import type { Expense } from "@gofin/core";

/**
 * Read the transaction and reporting money snapshot fields for an expense row.
 *
 * Every row stores explicit snapshot fields, so these helpers read them
 * directly.
 */

export function getTransactionAmount(expense: Expense): number {
  return expense.transactionAmount;
}

export function getTransactionCurrency(expense: Expense): string {
  return expense.transactionCurrency;
}

export function getReportingAmount(expense: Expense): number {
  return expense.reportingAmount;
}

export function getReportingCurrency(expense: Expense): string {
  return expense.reportingCurrency;
}

export function hasSameCurrencySnapshot(expense: Expense): boolean {
  return expense.transactionCurrency === expense.reportingCurrency;
}
