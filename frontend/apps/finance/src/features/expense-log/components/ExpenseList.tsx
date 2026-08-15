import type { Table } from "@tanstack/react-table";
import { formatAmount } from "@gofin/core";
import { Card, CardContent } from "@gofin/ui/components/card";
import type { ExpenseRow } from "../../../lib/expense-table-columns";

interface ExpenseListProps {
  table: Table<ExpenseRow>;
  onRowClick: (row: ExpenseRow) => void;
}

/**
 * Mobile list view for expenses. Renders a compact card with
 * name, date, and formatted amount. For foreign-currency rows,
 * shows the transaction amount and a smaller secondary reporting amount.
 * Visible on mobile only via CSS.
 */
export function ExpenseList({ table, onRowClick }: ExpenseListProps) {
  return (
    <Card>
      <CardContent className="p-0">
        <div className="divide-y">
          {table.getRowModel().rows.map((row) => {
            const expense = row.original;
            const transactionFormatted = formatAmount(
              expense.transactionAmountEffective,
              expense.transactionCurrencyEffective,
            );
            const className = expense.status === "corrected" ? "line-through" : "";

            return (
              <div
                key={row.id}
                className="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-muted/50"
                onClick={() => onRowClick(expense)}
                aria-label={`View expense: ${expense.name}`}
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onRowClick(expense);
                  }
                }}
              >
                <div className="flex flex-col gap-0.5">
                  <span
                    className={`text-sm font-medium ${className}`}
                  >
                    {expense.name}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {expense.expenseDate}
                  </span>
                  {expense.showReportingAmount && (
                    <span className={`text-xs text-muted-foreground ${className}`}>
                      {formatAmount(
                        expense.reportingAmountEffective,
                        expense.reportingCurrencyEffective,
                      )}
                    </span>
                  )}
                </div>
                <span
                  className={`text-sm font-semibold ${className}`}
                >
                  {transactionFormatted}
                </span>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}