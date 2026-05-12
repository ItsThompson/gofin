import type { Tag } from "../../../types";
import type { ExpenseFilters } from "../hooks/useExpenseFilters";
import { EXPENSE_TYPES } from "@gofin/core";
import { Input } from "@gofin/ui/components/input";
import { Card, CardContent } from "@gofin/ui/components/card";

interface FilterPanelProps {
  filters: ExpenseFilters;
  tags: Tag[];
}

/**
 * Renders expense type toggles, tag toggles, and date range inputs.
 * Visible when `filters.showFilters` is true (controlled by parent).
 */
export function FilterPanel({ filters, tags }: FilterPanelProps) {
  return (
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
                onClick={() => filters.toggleType(type)}
                className={`rounded-full border px-3 py-1 text-xs font-medium capitalize transition-colors ${
                  filters.criteria.selectedTypes.has(type)
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
                  onClick={() => filters.toggleTag(tag.id)}
                  className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                    filters.criteria.selectedTags.has(tag.id)
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
              value={filters.criteria.dateFrom}
              onChange={(event) => filters.setDateFrom(event.target.value)}
              className="h-8 w-auto text-xs"
              aria-label="Date from"
            />
            <span className="text-xs text-muted-foreground">to</span>
            <Input
              type="date"
              value={filters.criteria.dateTo}
              onChange={(event) => filters.setDateTo(event.target.value)}
              className="h-8 w-auto text-xs"
              aria-label="Date to"
            />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
