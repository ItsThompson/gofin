import { createColumnHelper } from "@tanstack/react-table";
import { formatAmount } from "@gofin/core";
import type { Expense, Tag } from "@gofin/core";

/** Enriched expense with resolved tag name and derived money display fields. */
export interface ExpenseRow extends Expense {
  tagName: string;
  /** Effective transaction amount (falls back to legacy amount). */
  transactionAmountEffective: number;
  /** Effective transaction currency (falls back to legacy currency or period currency). */
  transactionCurrencyEffective: string;
  /** Effective reporting amount (falls back to legacy amount). */
  reportingAmountEffective: number;
  /** Effective reporting currency (falls back to period reporting currency). */
  reportingCurrencyEffective: string;
  /** True when transaction currency differs from reporting currency. */
  showReportingAmount: boolean;
}

const columnHelper = createColumnHelper<ExpenseRow>();

/** Build table column definitions for the expense log. */
export function buildExpenseColumns() {
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
    columnHelper.accessor("reportingAmountEffective", {
      header: "Amount",
      cell: (info) => {
        const row = info.row.original;
        const transactionFormatted = formatAmount(
          row.transactionAmountEffective,
          row.transactionCurrencyEffective,
        );
        const className = row.status === "corrected" ? "line-through" : "";

        if (!row.showReportingAmount) {
          return (
            <span className={className}>{transactionFormatted}</span>
          );
        }

        // Foreign-currency row: show transaction amount and secondary reporting amount.
        const reportingFormatted = formatAmount(
          row.reportingAmountEffective,
          row.reportingCurrencyEffective,
        );
        return (
          <div className="flex flex-col">
            <span className={className}>{transactionFormatted}</span>
            <span className={`text-xs text-muted-foreground ${className}`}>
              {reportingFormatted}
            </span>
          </div>
        );
      },
      sortingFn: (rowA, rowB) =>
        (rowA.original?.reportingAmountEffective ?? 0) - (rowB.original?.reportingAmountEffective ?? 0),
      sortDescFirst: false,
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
 * Resolve tag names and derived money display fields by mapping tag IDs to tag
 * name strings and computing effective transaction/reporting amounts with
 * showReportingAmount. Falls back to legacy amount/currency for rows without
 * explicit money snapshots.
 *
 * @param expenses Raw expense data from the API.
 * @param tags Available tags for name resolution.
 * @param periodCurrency The selected period's reporting currency, used as a
 *   fallback for reportingCurrencyEffective when the expense lacks an explicit
 *   snapshot.
 */
export function resolveTagNames(
  expenses: Expense[],
  tags: Tag[],
  periodCurrency: string,
): ExpenseRow[] {
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));
  return expenses.map((expense) => {
    const transactionAmountEffective = expense.transactionAmount ?? expense.amount;
    const transactionCurrencyEffective =
      expense.transactionCurrency ?? expense.currency ?? periodCurrency;
    const reportingAmountEffective = expense.reportingAmount ?? expense.amount;
    const reportingCurrencyEffective =
      expense.reportingCurrency ?? periodCurrency;
    const showReportingAmount =
      transactionCurrencyEffective !== reportingCurrencyEffective;

    return {
      ...expense,
      tagName: tagMap.get(expense.tagId) ?? expense.tagId,
      transactionAmountEffective,
      transactionCurrencyEffective,
      reportingAmountEffective,
      reportingCurrencyEffective,
      showReportingAmount,
    };
  });
}