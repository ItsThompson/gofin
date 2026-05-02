import { useState, useEffect, useCallback, useMemo } from "react";
import { useNavigate } from "react-router";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  flexRender,
  type SortingState,
  type ColumnFiltersState,
} from "@tanstack/react-table";
import {
  apiClient,
  formatCurrency,
  type BudgetPeriod,
  type Expense,
  type PaginatedResponse,
  type Tag,
  type TagListResponse,
  type PeriodListResponse,
} from "@gofin/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Receipt,
  Loader2,
  ChevronUp,
  ChevronDown,
  ChevronsUpDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Filter,
  X,
} from "lucide-react";
import type { FinancePageProps } from "@/types";
import {
  buildExpenseColumns,
  resolveTagNames,
  type ExpenseRow,
} from "@/lib/expense-table-columns";

const EXPENSE_TYPES = ["essentials", "desires", "savings"] as const;
const PAGE_SIZE_OPTIONS = [10, 25, 50] as const;

/**
 * Expense log page: data table with TanStack Table.
 *
 * Fetches all expenses for a period, tags for name resolution, and
 * available periods for the period selector. Sorting, filtering, and
 * pagination are all client-side.
 */
export function ExpenseLogPage({ user }: FinancePageProps) {
  const navigate = useNavigate();
  const now = new Date();
  const [selectedYear, setSelectedYear] = useState(now.getFullYear());
  const [selectedMonth, setSelectedMonth] = useState(now.getMonth() + 1);

  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [periods, setPeriods] = useState<BudgetPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filter state
  const [showFilters, setShowFilters] = useState(false);
  const [selectedTypes, setSelectedTypes] = useState<Set<string>>(new Set());
  const [selectedTags, setSelectedTags] = useState<Set<string>>(new Set());
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  // Table state
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [expensesResponse, tagsResponse, periodsResponse] =
        await Promise.all([
          apiClient<PaginatedResponse<Expense>>(
            `/api/expenses?year=${selectedYear}&month=${selectedMonth}&page=1&pageSize=1000`,
          ),
          apiClient<TagListResponse>("/api/finance/tags").catch(
            (): TagListResponse => ({ tags: [] }),
          ),
          apiClient<PeriodListResponse>("/api/finance/periods").catch(
            (): PeriodListResponse => ({ periods: [] }),
          ),
        ]);

      setExpenses(expensesResponse.data);
      setTags(tagsResponse.tags);
      setPeriods(periodsResponse.periods);
    } catch {
      setError("Failed to load expenses. Please try again.");
    } finally {
      setLoading(false);
    }
  }, [selectedYear, selectedMonth]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Resolve tag names and apply custom filters
  const filteredData = useMemo(() => {
    const rows = resolveTagNames(expenses, tags);

    return rows.filter((row) => {
      if (selectedTypes.size > 0 && !selectedTypes.has(row.expenseType)) {
        return false;
      }
      if (selectedTags.size > 0 && !selectedTags.has(row.tagId)) {
        return false;
      }
      if (dateFrom && row.expenseDate < dateFrom) {
        return false;
      }
      if (dateTo && row.expenseDate > dateTo) {
        return false;
      }
      return true;
    });
  }, [expenses, tags, selectedTypes, selectedTags, dateFrom, dateTo]);

  const columns = useMemo(
    () => buildExpenseColumns(user.currency),
    [user.currency],
  );

  const table = useReactTable({
    data: filteredData,
    columns,
    state: { sorting, columnFilters },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: {
      pagination: { pageSize: 10 },
    },
  });

  function handlePeriodChange(value: string) {
    const [yearStr, monthStr] = value.split("-");
    setSelectedYear(Number(yearStr));
    setSelectedMonth(Number(monthStr));
  }

  function handleRowClick(row: ExpenseRow) {
    navigate(`/expenses/${row.id}`);
  }

  function toggleTypeFilter(type: string) {
    setSelectedTypes((prev) => {
      const next = new Set(prev);
      if (next.has(type)) {
        next.delete(type);
      } else {
        next.add(type);
      }
      return next;
    });
  }

  function toggleTagFilter(tagId: string) {
    setSelectedTags((prev) => {
      const next = new Set(prev);
      if (next.has(tagId)) {
        next.delete(tagId);
      } else {
        next.add(tagId);
      }
      return next;
    });
  }

  function clearFilters() {
    setSelectedTypes(new Set());
    setSelectedTags(new Set());
    setDateFrom("");
    setDateTo("");
  }

  const hasActiveFilters =
    selectedTypes.size > 0 ||
    selectedTags.size > 0 ||
    dateFrom !== "" ||
    dateTo !== "";

  const periodValue = `${selectedYear}-${selectedMonth}`;

  if (loading) {
    return (
      <div className="flex min-h-[300px] items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
        <span className="ml-2 text-muted-foreground">
          Loading expenses...
        </span>
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="mb-4 text-sm text-muted-foreground">{error}</p>
          <Button variant="outline" onClick={fetchData}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Receipt className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Expense Log</h1>
      </div>

      {/* Controls Row */}
      <div className="flex flex-wrap items-center gap-3">
        {/* Period Selector */}
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
            {periods.length > 0 ? (
              periods.map((period) => {
                const value = `${period.year}-${period.month}`;
                const label = new Date(
                  period.year,
                  period.month - 1,
                ).toLocaleString("en-US", {
                  month: "long",
                  year: "numeric",
                });
                return (
                  <option key={value} value={value}>
                    {label}
                  </option>
                );
              })
            ) : (
              <option value={periodValue}>
                {new Date(selectedYear, selectedMonth - 1).toLocaleString(
                  "en-US",
                  { month: "long", year: "numeric" },
                )}
              </option>
            )}
          </select>
        </div>

        {/* Filter Toggle */}
        <Button
          variant={showFilters ? "default" : "outline"}
          size="sm"
          onClick={() => setShowFilters(!showFilters)}
        >
          <Filter className="size-4" />
          Filters
          {hasActiveFilters && (
            <span className="ml-1 rounded-full bg-primary-foreground px-1.5 text-xs font-bold text-primary">
              {selectedTypes.size + selectedTags.size + (dateFrom ? 1 : 0) + (dateTo ? 1 : 0)}
            </span>
          )}
        </Button>

        {hasActiveFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <X className="size-4" />
            Clear filters
          </Button>
        )}

        {/* Total count */}
        <span className="ml-auto text-sm text-muted-foreground">
          {filteredData.length} expense{filteredData.length !== 1 ? "s" : ""}
        </span>
      </div>

      {/* Filter Panel */}
      {showFilters && (
        <Card>
          <CardContent className="flex flex-wrap gap-6 py-4">
            {/* Type filter */}
            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-medium text-muted-foreground">
                Expense Type
              </span>
              <div className="flex flex-wrap gap-2">
                {EXPENSE_TYPES.map((type) => (
                  <button
                    key={type}
                    type="button"
                    onClick={() => toggleTypeFilter(type)}
                    className={`rounded-full border px-3 py-1 text-xs font-medium capitalize transition-colors ${
                      selectedTypes.has(type)
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-input bg-transparent hover:bg-muted"
                    }`}
                  >
                    {type}
                  </button>
                ))}
              </div>
            </div>

            {/* Tag filter */}
            {tags.length > 0 && (
              <div className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">
                  Tag
                </span>
                <div className="flex flex-wrap gap-2">
                  {tags.map((tag) => (
                    <button
                      key={tag.id}
                      type="button"
                      onClick={() => toggleTagFilter(tag.id)}
                      className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                        selectedTags.has(tag.id)
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-input bg-transparent hover:bg-muted"
                      }`}
                    >
                      {tag.name}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Date range filter */}
            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-medium text-muted-foreground">
                Date Range
              </span>
              <div className="flex items-center gap-2">
                <Input
                  type="date"
                  value={dateFrom}
                  onChange={(event) => setDateFrom(event.target.value)}
                  className="h-8 w-auto text-xs"
                  aria-label="Date from"
                />
                <span className="text-xs text-muted-foreground">to</span>
                <Input
                  type="date"
                  value={dateTo}
                  onChange={(event) => setDateTo(event.target.value)}
                  className="h-8 w-auto text-xs"
                  aria-label="Date to"
                />
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Table / List */}
      {filteredData.length === 0 ? (
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
            <Card>
              <CardContent className="p-0">
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      {table.getHeaderGroups().map((headerGroup) => (
                        <tr key={headerGroup.id} className="border-b">
                          {headerGroup.headers.map((header) => (
                            <th
                              key={header.id}
                              className="px-4 py-3 text-left font-medium text-muted-foreground"
                            >
                              {header.isPlaceholder ? null : (
                                <button
                                  type="button"
                                  className={`inline-flex items-center gap-1 ${
                                    header.column.getCanSort()
                                      ? "cursor-pointer select-none hover:text-foreground"
                                      : ""
                                  }`}
                                  onClick={header.column.getToggleSortingHandler()}
                                >
                                  {flexRender(
                                    header.column.columnDef.header,
                                    header.getContext(),
                                  )}
                                  <SortIndicator
                                    sorted={header.column.getIsSorted()}
                                    canSort={header.column.getCanSort()}
                                  />
                                </button>
                              )}
                            </th>
                          ))}
                        </tr>
                      ))}
                    </thead>
                    <tbody>
                      {table.getRowModel().rows.map((row) => (
                        <tr
                          key={row.id}
                          className="cursor-pointer border-b transition-colors last:border-0 hover:bg-muted/50"
                          onClick={() => handleRowClick(row.original)}
                          aria-label={`View expense: ${row.original.name}`}
                          tabIndex={0}
                          onKeyDown={(event) => {
                            if (event.key === "Enter" || event.key === " ") {
                              event.preventDefault();
                              handleRowClick(row.original);
                            }
                          }}
                        >
                          {row.getVisibleCells().map((cell) => (
                            <td key={cell.id} className="px-4 py-3">
                              {flexRender(
                                cell.column.columnDef.cell,
                                cell.getContext(),
                              )}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Mobile List (visible on mobile only) */}
          <div className="md:hidden">
            <Card>
              <CardContent className="p-0">
                <div className="divide-y">
                  {table.getRowModel().rows.map((row) => {
                    const expense = row.original;
                    return (
                      <div
                        key={row.id}
                        className="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-muted/50"
                        onClick={() => handleRowClick(expense)}
                        aria-label={`View expense: ${expense.name}`}
                        tabIndex={0}
                        onKeyDown={(event) => {
                          if (
                            event.key === "Enter" ||
                            event.key === " "
                          ) {
                            event.preventDefault();
                            handleRowClick(expense);
                          }
                        }}
                      >
                        <div className="flex flex-col gap-0.5">
                          <span
                            className={`text-sm font-medium ${
                              expense.status === "corrected"
                                ? "line-through"
                                : ""
                            }`}
                          >
                            {expense.name}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {expense.expenseDate}
                          </span>
                        </div>
                        <span
                          className={`text-sm font-semibold ${
                            expense.status === "corrected"
                              ? "line-through"
                              : ""
                          }`}
                        >
                          {formatCurrency(expense.amount, user.currency)}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Pagination */}
          <PaginationControls table={table} />
        </>
      )}
    </div>
  );
}

// --- Sub-components ---

function SortIndicator({
  sorted,
  canSort,
}: {
  sorted: false | "asc" | "desc";
  canSort: boolean;
}) {
  if (!canSort) return null;

  if (sorted === "asc") {
    return <ChevronUp className="size-3.5" />;
  }
  if (sorted === "desc") {
    return <ChevronDown className="size-3.5" />;
  }
  return <ChevronsUpDown className="size-3.5 text-muted-foreground/50" />;
}

function PaginationControls({
  table,
}: {
  table: ReturnType<typeof useReactTable<ExpenseRow>>;
}) {
  const pageIndex = table.getState().pagination.pageIndex;
  const pageCount = table.getPageCount();
  const pageSize = table.getState().pagination.pageSize;
  const totalRows = table.getFilteredRowModel().rows.length;

  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">Rows per page:</span>
        <select
          value={pageSize}
          onChange={(event) =>
            table.setPageSize(Number(event.target.value))
          }
          className="h-8 rounded-lg border border-input bg-transparent px-2 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          aria-label="Page size"
        >
          {PAGE_SIZE_OPTIONS.map((size) => (
            <option key={size} value={size}>
              {size}
            </option>
          ))}
        </select>
      </div>

      <span className="text-sm text-muted-foreground">
        {totalRows === 0
          ? "0 of 0"
          : `${pageIndex * pageSize + 1}–${Math.min(
              (pageIndex + 1) * pageSize,
              totalRows,
            )} of ${totalRows}`}
      </span>

      <div className="flex items-center gap-1">
        <Button
          variant="outline"
          size="icon"
          className="size-8"
          onClick={() => table.setPageIndex(0)}
          disabled={!table.getCanPreviousPage()}
          aria-label="First page"
        >
          <ChevronsLeft className="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon"
          className="size-8"
          onClick={() => table.previousPage()}
          disabled={!table.getCanPreviousPage()}
          aria-label="Previous page"
        >
          <ChevronLeft className="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon"
          className="size-8"
          onClick={() => table.nextPage()}
          disabled={!table.getCanNextPage()}
          aria-label="Next page"
        >
          <ChevronRight className="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon"
          className="size-8"
          onClick={() => table.setPageIndex(pageCount - 1)}
          disabled={!table.getCanNextPage()}
          aria-label="Last page"
        >
          <ChevronsRight className="size-4" />
        </Button>
      </div>
    </div>
  );
}
