import { getMinorUnitDigits, toMajorUnits } from "@gofin/core";
import type { Tag } from "@gofin/core";
import type { ExpenseSuggestion, ExpenseSuggestionPatch } from "./types";

function formatMinorUnits(amount: number, currency: string): string {
  return toMajorUnits(amount, currency).toFixed(getMinorUnitDigits(currency));
}

/** Resolve the canonical transaction amount, falling back to the deprecated amount field. */
function resolveTransactionAmount(suggestion: ExpenseSuggestion): number {
  return suggestion.transactionAmount ?? suggestion.amount;
}

/** Resolve the canonical transaction currency, falling back to the deprecated currency field. */
function resolveTransactionCurrency(suggestion: ExpenseSuggestion): string {
  return suggestion.transactionCurrency ?? suggestion.currency;
}

export function createExpenseSuggestionPatch(
  suggestion: ExpenseSuggestion,
  tags: Tag[],
): ExpenseSuggestionPatch {
  const hasValidTag = tags.some((tag) => tag.id === suggestion.tagId);
  const amount = resolveTransactionAmount(suggestion);
  const currency = resolveTransactionCurrency(suggestion);

  return {
    name: suggestion.name,
    amountDollars: formatMinorUnits(amount, currency),
    currency,
    expenseType: suggestion.expenseType,
    tagId: hasValidTag ? suggestion.tagId : null,
  };
}
