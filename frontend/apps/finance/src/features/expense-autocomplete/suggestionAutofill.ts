import { getMinorUnitDigits, toMajorUnits } from "@gofin/core";
import type { Tag } from "@gofin/core";
import type { ExpenseSuggestion, ExpenseSuggestionPatch } from "./types";

function formatMinorUnits(amount: number, currency: string): string {
  return toMajorUnits(amount, currency).toFixed(getMinorUnitDigits(currency));
}

export function createExpenseSuggestionPatch(
  suggestion: ExpenseSuggestion,
  tags: Tag[],
): ExpenseSuggestionPatch {
  const hasValidTag = tags.some((tag) => tag.id === suggestion.tagId);

  return {
    name: suggestion.name,
    amountDollars: formatMinorUnits(
      suggestion.originalTransactionAmountInMinorUnits,
      suggestion.transactionCurrencyCode,
    ),
    currency: suggestion.transactionCurrencyCode,
    expenseType: suggestion.expenseType,
    tagId: hasValidTag ? suggestion.tagId : null,
  };
}
