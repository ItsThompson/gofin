import type { Expense } from "@gofin/core";

/**
 * Whether the expense's transaction and reporting currency snapshots match.
 */
export function hasSameCurrencySnapshot(expense: Expense): boolean {
  return expense.transactionCurrencyCode === expense.reportingCurrencyCode;
}
