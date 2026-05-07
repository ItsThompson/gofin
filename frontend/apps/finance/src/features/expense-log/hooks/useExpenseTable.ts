import { useState, useMemo } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  type SortingState,
  type ColumnFiltersState,
  type Table,
} from "@tanstack/react-table";
import { buildExpenseColumns, type ExpenseRow } from "@/lib/expense-table-columns";

export interface ExpenseTableState {
  table: Table<ExpenseRow>;
}

/**
 * Manages TanStack Table state: sorting, column filters, and pagination.
 * Returns the table instance to be passed to presentation components.
 */
export function useExpenseTable(
  data: ExpenseRow[],
  currency: string,
): ExpenseTableState {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  const columns = useMemo(
    () => buildExpenseColumns(currency),
    [currency],
  );

  const table = useReactTable({
    data,
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

  return { table };
}
