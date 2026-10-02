import { ApiRequestError } from "@gofin/api";
import type { BudgetPeriod } from "@gofin/core";

export function getPeriodErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) return error.message;
  if (error instanceof Error) return error.message;
  return "This period is unavailable right now.";
}

export function isSamePeriod(
  first: BudgetPeriod,
  second: BudgetPeriod,
): boolean {
  return (
    first.id === second.id &&
    first.userId === second.userId &&
    first.year === second.year &&
    first.month === second.month &&
    first.budgetAmount === second.budgetAmount &&
    first.reportingCurrencyCode === second.reportingCurrencyCode &&
    first.essentialsPercent === second.essentialsPercent &&
    first.desiresPercent === second.desiresPercent &&
    first.savingsPercent === second.savingsPercent &&
    first.createdAt === second.createdAt &&
    first.updatedAt === second.updatedAt
  );
}
