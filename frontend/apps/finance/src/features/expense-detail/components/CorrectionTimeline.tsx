import { formatCurrency } from "@gofin/core";
import type { Expense, Tag } from "@gofin/core";
import { ArrowRight } from "lucide-react";
import { computeChanges } from "../utils/computeChanges";

interface CorrectionTimelineProps {
  entries: Expense[];
  currency: string;
  tags: Tag[];
  currentExpenseId: string;
}

export function CorrectionTimeline({
  entries,
  currency,
  tags,
  currentExpenseId,
}: CorrectionTimelineProps) {
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));

  return (
    <div className="space-y-3">
      {entries.map((entry, index) => {
        const previous = index > 0 ? entries[index - 1] : null;
        const isCurrent = entry.id === currentExpenseId;
        const changes = previous
          ? computeChanges(previous, entry, tags, currency)
          : [];

        return (
          <div
            key={entry.id}
            className={`rounded-lg border p-3 text-sm ${
              isCurrent
                ? "border-primary/50 bg-primary/5"
                : "border-border bg-muted/30"
            }`}
          >
            <div className="mb-1 flex items-center justify-between">
              <span className="font-medium">
                {index === 0 ? "Original" : `Correction ${index}`}
                {isCurrent && (
                  <span className="ml-2 text-xs text-primary">(viewing)</span>
                )}
              </span>
              <span
                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                  entry.status === "active"
                    ? "bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-200"
                    : "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-200"
                }`}
              >
                {entry.status === "active" ? "Active" : "Corrected"}
              </span>
            </div>
            <div className="text-muted-foreground">
              {entry.name} · {formatCurrency(entry.amount, currency)} ·{" "}
              {entry.expenseType} · {tagMap.get(entry.tagId) ?? entry.tagId}
            </div>
            {changes.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-1">
                {changes.map((change) => (
                  <span
                    key={change.field}
                    className="inline-flex items-center gap-1 rounded bg-blue-100 px-1.5 py-0.5 text-xs text-blue-800 dark:bg-blue-900/50 dark:text-blue-200"
                  >
                    <ArrowRight className="size-3" />
                    {change.field}: {change.from} → {change.to}
                  </span>
                ))}
              </div>
            )}
            <div className="mt-1 text-xs text-muted-foreground/70">
              {entry.createdAt}
            </div>
          </div>
        );
      })}
    </div>
  );
}
