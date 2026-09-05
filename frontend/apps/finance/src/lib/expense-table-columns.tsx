import { createColumnHelper } from "@tanstack/react-table";
import { formatAmount } from "@gofin/core";
import type { Expense, Tag } from "@gofin/core";

export interface ExpenseRow extends Expense {
  tagName: string;
  transactionAmountEffective: number;
  transactionCurrencyEffective: string;
  reportingAmountEffective: number;
  reportingCurrencyEffective: string;
  /** True when transaction currency differs from reporting currency. */
  showReportingAmount: boolean;
}

const columnHelper = createColumnHelper<ExpenseRow>();

export function buildExpenseColumns() {
  return [
    columnHelper.accessor("expenseDateIso", {
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
            <span
              className={`text-xs text-muted-foreground ${className}`}
              aria-label={`Budget impact: ${reportingFormatted}`}
            >
              Budget impact: {reportingFormatted}
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

export function resolveTagNames(
  expenses: Expense[],
  tags: Tag[],
): ExpenseRow[] {
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));
  return expenses.map((expense) => {
    const transactionAmountEffective = expense.originalTransactionAmountInMinorUnits;
    const transactionCurrencyEffective = expense.transactionCurrencyCode;
    const reportingAmountEffective = expense.reportingAmountInMinorUnits;
    const reportingCurrencyEffective = expense.reportingCurrencyCode;
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