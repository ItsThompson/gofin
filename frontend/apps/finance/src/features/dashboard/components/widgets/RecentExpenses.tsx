import { Link } from "react-router";
import { formatCurrency } from "@gofin/core";
import type { Expense } from "@gofin/core";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";

interface RecentExpensesProps {
  expenses: Expense[];
  currency: string;
}

/**
 * Recent expense list on the dashboard. Each row shows the budget-impact
 * (reporting) amount formatted in the period reporting currency, not the
 * transaction amount. For legacy rows without a reportingAmount, the amount
 * field is used as a fallback (it equals the reporting amount for same-currency
 * legacy rows).
 */
export function RecentExpenses({ expenses, currency }: RecentExpensesProps) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Recent Expenses</CardTitle>
          <Link
            to="/expenses"
            className="text-sm text-primary hover:underline"
          >
            View All
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        <div className="divide-y">
          {expenses.map((expense) => {
            const reportingAmount =
              expense.reportingAmount ?? expense.amount;
            return (
              <div
                key={expense.id}
                className="flex items-center justify-between py-3 first:pt-0 last:pb-0"
              >
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium">{expense.name}</span>
                  <span className="text-xs text-muted-foreground">
                    {expense.expenseDate}
                  </span>
                </div>
                <span className="text-sm font-semibold">
                  {formatCurrency(reportingAmount, currency)}
                </span>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}