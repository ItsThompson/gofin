import type { Tag } from "@gofin/core";
import type { ExpenseSuggestion, ExpenseSuggestionPatch } from "./types";

function formatMinorUnits(amount: number): string {
  return (amount / 100).toFixed(2);
}

export function createExpenseSuggestionPatch(
  suggestion: ExpenseSuggestion,
  tags: Tag[],
): ExpenseSuggestionPatch {
  const hasValidTag = tags.some((tag) => tag.id === suggestion.tagId);

  return {
    name: suggestion.name,
    amountDollars: formatMinorUnits(suggestion.amount),
    expenseType: suggestion.expenseType,
    tagId: hasValidTag ? suggestion.tagId : null,
  };
}
