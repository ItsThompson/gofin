import { useState } from "react";
import { Receipt, Filter, X } from "lucide-react";
import { Button } from "@gofin/ui/components/button";
import { Card, CardContent } from "@gofin/ui/components/card";
import { ExpenseLogSkeleton } from "@gofin/ui/components/skeletons";
import type { FinancePageProps } from "../../types";
import type { ExpenseRow } from "../../lib/expense-table-columns";
import { useExpenseFilters } from "./hooks/useExpenseFilters";
import { useExpenseLogData } from "./hooks/useExpenseLogData";
import { useExpenseTable } from "./hooks/useExpenseTable";
import { FilterPanel } from "./components/FilterPanel";
import { ExpenseTable } from "./components/ExpenseTable";
import { ExpenseList } from "./components/ExpenseList";
import { PaginationControls } from "./components/PaginationControls";
import { ExpenseDetailModal } from "../expense-detail";

/**
 * Expense log feature orchestrator. Composes filter, data, and table hooks
 * and renders the appropriate view components.
 */
export function ExpenseLogFeature({ user }: FinancePageProps) {
  const now = new Date();
  const filters = useExpenseFilters();
  const data = useExpenseLogData(filters.criteria);
  const { table } = useExpenseTable(data.expenses, user.currency);

  const [selectedExpenseId, setSelectedExpenseId] = useState<string | null>(null);

  function handlePeriodChange(value: string) {
    const [yearStr, monthStr] = value.split("-");
    data.setSelectedYear(Number(yearStr));
    data.setSelectedMonth(Number(monthStr));
  }

  function handleRowClick(row: ExpenseRow) {
    setSelectedExpenseId(row.id);
  }

  if (data.loading) {
    return <ExpenseLogSkeleton />;
  }

  const periodValue = `${data.selectedYear}-${data.selectedMonth}`;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Receipt className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Expense Log</h1>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <label htmlFor="period-select" className="text-sm font-medium">
            Period:
          </label>
          <select
            id="period-select"
            value={periodValue}
            onChange={(event) => handlePeriodChange(event.target.value)}
            className="h-8 rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {data.periods.length > 0 ? (
              data.periods.map((period) => {
                const optionValue = `${period.year}-${period.month}`;
                const label = new Date(
                  period.year,
                  period.month - 1,
                ).toLocaleString("en-US", {
                  month: "long",
                  year: "numeric",
                });
                return (
                  <option key={optionValue} value={optionValue}>
                    {label}
                  </option>
                );
              })
            ) : (
              <option value={periodValue}>
                {new Date(data.selectedYear, data.selectedMonth - 1).toLocaleString(
                  "en-US",
                  { month: "long", year: "numeric" },
                )}
              </option>
            )}
          </select>
        </div>

        <Button
          variant={filters.showFilters ? "default" : "outline"}
          size="sm"
          onClick={filters.toggleFilters}
        >
          <Filter className="size-4" />
          Filters
          {filters.hasActiveFilters && (
            <span className="ml-1 rounded-full bg-primary-foreground px-1.5 text-xs font-bold text-primary">
              {filters.criteria.selectedTypes.size +
                filters.criteria.selectedTags.size +
                (filters.criteria.dateFrom ? 1 : 0) +
                (filters.criteria.dateTo ? 1 : 0)}
            </span>
          )}
        </Button>

        {filters.hasActiveFilters && (
          <Button variant="ghost" size="sm" onClick={filters.clearFilters}>
            <X className="size-4" />
            Clear filters
          </Button>
        )}

        <span className="ml-auto text-sm text-muted-foreground">
          {data.expenses.length} expense{data.expenses.length !== 1 ? "s" : ""}
        </span>
      </div>

      {filters.showFilters && (
        <FilterPanel filters={filters} tags={data.tags} />
      )}

      {data.expenses.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <Receipt className="mb-4 size-12 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">
              No expenses for this period
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Desktop Table (hidden on mobile) */}
          <div className="hidden md:block">
            <ExpenseTable table={table} onRowClick={handleRowClick} />
          </div>

          {/* Mobile List (visible on mobile only) */}
          <div className="md:hidden">
            <ExpenseList
              table={table}
              currency={user.currency}
              onRowClick={handleRowClick}
            />
          </div>

          <PaginationControls table={table} />
        </>
      )}

      <ExpenseDetailModal
        expenseId={selectedExpenseId}
        currency={user.currency}
        tags={data.tags}
        currentYear={now.getFullYear()}
        currentMonth={now.getMonth() + 1}
        onClose={() => setSelectedExpenseId(null)}
        onCorrected={() => data.refresh()}
      />
    </div>
  );
}
