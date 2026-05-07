import { formatCurrency } from "@gofin/core";
import type { Expense, Tag } from "@/types";
import { History, Pencil } from "lucide-react";
import { Button } from "@gofin/ui/components/button";
import { CorrectionTimeline } from "./CorrectionTimeline";

interface DetailViewProps {
  expense: Expense;
  currency: string;
  tags: Tag[];
  history: Expense[];
  proRataGroup: Expense[];
  currentYear: number;
  currentMonth: number;
  onCorrectClick: () => void;
}

export function DetailView({
  expense,
  currency,
  tags,
  history,
  proRataGroup,
  currentYear,
  currentMonth,
  onCorrectClick,
}: DetailViewProps) {
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));

  const isCurrentPeriod =
    expense.periodYear === currentYear &&
    expense.periodMonth === currentMonth;
  const canCorrect = expense.status === "active" && isCurrentPeriod;
  const hasCorrections = history.length > 1;

  // Find the correction that supersedes this expense (if corrected)
  const correctedBy =
    expense.status === "corrected"
      ? history.find((entry) => entry.correctsId === expense.id)
      : null;

  // Find the expense this one corrects (if it's a correction)
  const correctsEntry = expense.correctsId
    ? history.find((entry) => entry.id === expense.correctsId)
    : null;

  return (
    <div className="space-y-4">
      {/* Correction Notices */}
      {correctedBy && (
        <div className="rounded-lg bg-yellow-100 p-3 text-sm text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-200">
          This expense was corrected. See correction: {correctedBy.name} (
          {formatCurrency(correctedBy.amount, currency)})
        </div>
      )}

      {correctsEntry && (
        <div className="rounded-lg bg-blue-100 p-3 text-sm text-blue-800 dark:bg-blue-900/30 dark:text-blue-200">
          This corrects expense: {correctsEntry.name} (
          {formatCurrency(correctsEntry.amount, currency)})
        </div>
      )}

      {/* Detail Fields */}
      <div className="grid gap-3">
        <DetailField label="Name" value={expense.name} />
        <DetailField
          label="Amount"
          value={formatCurrency(expense.amount, currency)}
        />
        <DetailField
          label="Type"
          value={
            <span className="capitalize">{expense.expenseType}</span>
          }
        />
        <DetailField
          label="Tag"
          value={tagMap.get(expense.tagId) ?? expense.tagId}
        />
        <DetailField label="Date" value={expense.expenseDate} />
        <DetailField
          label="Period"
          value={`${expense.periodYear}-${String(expense.periodMonth).padStart(2, "0")}`}
        />
        <DetailField label="Currency" value={expense.currency} />
        <DetailField label="Created" value={expense.createdAt} />
        <DetailField
          label="Status"
          value={
            expense.status === "corrected" ? (
              <span className="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-200">
                Corrected
              </span>
            ) : (
              <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900/50 dark:text-green-200">
                Active
              </span>
            )
          }
        />
      </div>

      {/* Pro-rata Metadata */}
      {expense.isProRata && (
        <div className="border-t pt-4">
          <div className="mb-3 text-sm font-medium">Pro-rata Details</div>
          <div className="grid gap-2 text-sm">
            {expense.proRataTotal && (
              <DetailField
                label="Installment"
                value={`${expense.proRataIndex} of ${expense.proRataTotal}`}
              />
            )}
            {proRataGroup.length > 0 && proRataGroup.length === expense.proRataTotal && (
              <DetailField
                label="Total Amount (all installments)"
                value={formatCurrency(
                  proRataGroup.reduce((sum, entry) => sum + entry.amount, 0),
                  currency,
                )}
              />
            )}
          </div>
          {proRataGroup.length > 1 && (
            <div className="mt-3">
              <div className="mb-2 text-xs text-muted-foreground">Related Installments</div>
              <div className="space-y-1">
                {proRataGroup.map((entry) => (
                  <div
                    key={entry.id}
                    className={`flex items-center justify-between rounded px-2 py-1.5 text-sm ${
                      entry.id === expense.id
                        ? "bg-primary/5 font-medium"
                        : "bg-muted/30"
                    }`}
                  >
                    <span>
                      {entry.proRataIndex} of {entry.proRataTotal}
                      {entry.id === expense.id && " (current)"}
                    </span>
                    <span className="text-muted-foreground">
                      {formatCurrency(entry.amount, currency)} · {entry.expenseDate}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Correction History Timeline */}
      {hasCorrections && (
        <div className="border-t pt-4">
          <div className="mb-3 flex items-center gap-2 text-sm font-medium">
            <History className="size-4" />
            Correction History
          </div>
          <CorrectionTimeline
            entries={history}
            currency={currency}
            tags={tags}
            currentExpenseId={expense.id}
          />
        </div>
      )}

      {/* Correct Button */}
      {canCorrect && (
        <div className="border-t pt-4">
          <Button onClick={onCorrectClick} className="w-full">
            <Pencil className="size-4" />
            Correct This Expense
          </Button>
        </div>
      )}
    </div>
  );
}

function DetailField({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  );
}
