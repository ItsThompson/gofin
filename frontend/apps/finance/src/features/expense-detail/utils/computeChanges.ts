import { formatCurrency } from "@gofin/core";
import type { Expense, Tag, ExpenseType } from "@gofin/core";

export interface ExpenseChange {
  field: string;
  from: string;
  to: string;
}

interface CorrectionValues {
  name: string;
  transactionAmount: number;
  expenseType: ExpenseType;
  tagId: string;
  expenseDate: string;
}

/**
 * Compare an expense's current values against correction form values.
 * Returns a list of changed fields for display in the confirmation view.
 * Pure function: no side effects, easily unit testable.
 */
export function computeChanges(
  original: Expense,
  corrected: CorrectionValues,
  tags: Tag[],
  currency: string,
): ExpenseChange[] {
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));
  const changes: ExpenseChange[] = [];

  if (original.name !== corrected.name) {
    changes.push({ field: "Name", from: original.name, to: corrected.name });
  }

  if (original.transactionAmount !== corrected.transactionAmount) {
    changes.push({
      field: "Amount",
      from: formatCurrency(original.transactionAmount, currency),
      to: formatCurrency(corrected.transactionAmount, currency),
    });
  }

  if (original.expenseType !== corrected.expenseType) {
    changes.push({
      field: "Type",
      from: original.expenseType,
      to: corrected.expenseType,
    });
  }

  if (original.tagId !== corrected.tagId) {
    changes.push({
      field: "Tag",
      from: tagMap.get(original.tagId) ?? original.tagId,
      to: tagMap.get(corrected.tagId) ?? corrected.tagId,
    });
  }

  if (original.expenseDate !== corrected.expenseDate) {
    changes.push({
      field: "Date",
      from: original.expenseDate,
      to: corrected.expenseDate,
    });
  }

  return changes;
}
