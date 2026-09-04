import { useState } from "react";
import { formatCurrency } from "@gofin/core";
import type { Expense, Tag } from "@gofin/core";
import { History, Pencil, Trash2 } from "lucide-react";
import { Button } from "@gofin/ui/components/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
} from "@gofin/ui/components/dialog";
import { CorrectionTimeline } from "./CorrectionTimeline";
import { hasSameCurrencySnapshot } from "../utils/moneyFacts";

interface DetailViewProps {
  expense: Expense;
  currency: string;
  tags: Tag[];
  history: Expense[];
  proRataGroup: Expense[];
  currentYear: number;
  currentMonth: number;
  onCorrectClick: () => void;
  onDeleteClick: () => void;
  deleting: boolean;
  deleteError: string | null;
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
  onDeleteClick,
  deleting,
  deleteError,
}: DetailViewProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));

  const isCurrentPeriod =
    expense.periodYear === currentYear &&
    expense.periodMonth === currentMonth;
  const canCorrect = expense.status === "active" && isCurrentPeriod;
  const hasCorrections = history.length > 1;

  const correctedBy =
    expense.status === "corrected"
      ? history.find((entry) => entry.correctsId === expense.id)
      : null;

  const correctsEntry = expense.correctsId
    ? history.find((entry) => entry.id === expense.correctsId)
    : null;

  const transactionCurrencyCode = expense.transactionCurrencyCode;
  const originalTransactionAmountInMinorUnits = expense.originalTransactionAmountInMinorUnits;
  const reportingCurrencyCode = expense.reportingCurrencyCode;
  const reportingAmountInMinorUnits = expense.reportingAmountInMinorUnits;
  const sameCurrency = hasSameCurrencySnapshot(expense);

  return (
    <div className="space-y-4">
      {correctedBy && (
        <div className="rounded-lg bg-yellow-100 p-3 text-sm text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-200">
          This expense was corrected. See correction: {correctedBy.name} (
          {formatCurrency(
            correctedBy.originalTransactionAmountInMinorUnits,
            correctedBy.transactionCurrencyCode,
          )}
          )
        </div>
      )}

      {correctsEntry && (
        <div className="rounded-lg bg-blue-100 p-3 text-sm text-blue-800 dark:bg-blue-900/30 dark:text-blue-200">
          This corrects expense: {correctsEntry.name} (
          {formatCurrency(
            correctsEntry.originalTransactionAmountInMinorUnits,
            correctsEntry.transactionCurrencyCode,
          )}
          )
        </div>
      )}

      <div className="grid gap-3">
        <DetailField label="Name" value={expense.name} />
        {sameCurrency ? (
          <DetailField
            label="Period Amount"
            value={formatCurrency(reportingAmountInMinorUnits, reportingCurrencyCode)}
          />
        ) : (
          <>
            <DetailField
              label="Transaction Amount"
              value={formatCurrency(originalTransactionAmountInMinorUnits, transactionCurrencyCode)}
            />
            <DetailField
              label="Budget Impact"
              value={formatCurrency(reportingAmountInMinorUnits, reportingCurrencyCode)}
            />
            {expense.sourceToTargetExchangeRate && (
              <DetailField label="Exchange Rate" value={expense.sourceToTargetExchangeRate} />
            )}
            {expense.exchangeRateTimestamp && (
              <DetailField
                label="Rate Timestamp"
                value={expense.exchangeRateTimestamp}
              />
            )}
          </>
        )}
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
        <DetailField label="Date" value={expense.expenseDateIso} />
        <DetailField
          label="Period"
          value={`${expense.periodYear}-${String(expense.periodMonth).padStart(2, "0")}`}
        />
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
                  proRataGroup.reduce((sum, entry) => sum + entry.originalTransactionAmountInMinorUnits, 0),
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
                      {formatCurrency(entry.originalTransactionAmountInMinorUnits, currency)} · {entry.expenseDateIso}
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

      {canCorrect && (
        <div className="border-t pt-4">
          <Button onClick={onCorrectClick} className="w-full">
            <Pencil className="size-4" />
            Correct This Expense
          </Button>
        </div>
      )}

      {canCorrect && (
        <div className="pt-2">
          <Button
            variant="destructive"
            className="w-full"
            onClick={() => setConfirmDelete(true)}
          >
            <Trash2 className="size-4" />
            Delete
          </Button>
        </div>
      )}

      {confirmDelete && (
        <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delete Expense</DialogTitle>
              <DialogClose onClick={() => setConfirmDelete(false)} />
            </DialogHeader>
            <p className="text-sm text-muted-foreground">
              Are you sure you want to delete this expense? This cannot be undone from the UI.
            </p>
            {deleteError && (
              <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
                {deleteError}
              </div>
            )}
            <div className="flex justify-end gap-2 pt-2">
              <Button
                variant="outline"
                onClick={() => setConfirmDelete(false)}
                disabled={deleting}
              >
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  setConfirmDelete(false);
                  onDeleteClick();
                }}
                disabled={deleting}
              >
                {deleting ? "Deleting..." : "Delete"}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
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
