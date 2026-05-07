import {
  flexRender,
  type Table,
} from "@tanstack/react-table";
import { Card, CardContent } from "@gofin/ui/components/card";
import { ChevronUp, ChevronDown, ChevronsUpDown } from "lucide-react";
import type { ExpenseRow } from "../../../lib/expense-table-columns";

interface ExpenseTableProps {
  table: Table<ExpenseRow>;
  onRowClick: (row: ExpenseRow) => void;
}

/**
 * Desktop table view for expenses using TanStack Table.
 * Hidden on mobile via CSS class applied by parent.
 */
export function ExpenseTable({ table, onRowClick }: ExpenseTableProps) {
  return (
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
                  onClick={() => onRowClick(row.original)}
                  aria-label={`View expense: ${row.original.name}`}
                  tabIndex={0}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onRowClick(row.original);
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
  );
}

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
