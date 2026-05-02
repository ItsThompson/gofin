import { createColumnHelper } from "@tanstack/react-table";
import { formatCurrency } from "@gofin/types";
import type { Expense, Tag } from "@gofin/types";

/** Enriched expense with resolved tag name for display. */
export interface ExpenseRow extends Expense {
  tagName: string;
}

const columnHelper = createColumnHelper<ExpenseRow>();

/** Build table column definitions for the expense log. */
export function buildExpenseColumns(currency: string) {
  return [
    columnHelper.accessor("expenseDate", {
      header: "Date",
      cell: (info) => info.getValue(),
      sortingFn: "alphanumeric",
    }),
    columnHelper.accessor("name", {
      header: "Name",
      cell: (info) => {
        const row = info.row.original;
        return (
          <span className={row.status === "corrected" ? "line-through" : ""}>
            {info.getValue()}
          </span>
        );
      },
    }),
    columnHelper.accessor("amount", {
      header: "Amount",
      cell: (info) => {
        const row = info.row.original;
        return (
          <span className={row.status === "corrected" ? "line-through" : ""}>
            {formatCurrency(info.getValue(), currency)}
          </span>
        );
      },
      sortingFn: "basic",
    }),
    columnHelper.accessor("expenseType", {
      header: "Type",
      cell: (info) => (
        <span className="capitalize">{info.getValue()}</span>
      ),
    }),
    columnHelper.accessor("tagName", {
      header: "Tag",
      cell: (info) => info.getValue(),
    }),
    columnHelper.accessor("status", {
      header: "Status",
      cell: (info) => {
        const value = info.getValue();
        if (value === "corrected") {
          return (
            <span className="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-200">
              Corrected
            </span>
          );
        }
        return (
          <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900/50 dark:text-green-200">
            Active
          </span>
        );
      },
      enableSorting: false,
    }),
  ];
}

/**
 * Resolve tag names by mapping tag IDs to tag name strings.
 * Falls back to the tag ID if the tag is not found.
 */
export function resolveTagNames(
  expenses: Expense[],
  tags: Tag[],
): ExpenseRow[] {
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));
  return expenses.map((expense) => ({
    ...expense,
    tagName: tagMap.get(expense.tagId) ?? expense.tagId,
  }));
}
