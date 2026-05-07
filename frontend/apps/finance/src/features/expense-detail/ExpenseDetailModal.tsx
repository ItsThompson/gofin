import type { Tag } from "@/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
} from "@gofin/ui/components/dialog";
import { Loader2, AlertCircle } from "lucide-react";
import { useExpenseDetail } from "./hooks/useExpenseDetail";
import { DetailView } from "./components/DetailView";
import { CorrectionForm } from "./components/CorrectionForm";

interface ExpenseDetailModalProps {
  expenseId: string | null;
  currency: string;
  tags: Tag[];
  currentYear: number;
  currentMonth: number;
  onClose: () => void;
  /** Called after a successful correction so the parent can refresh data. */
  onCorrected: () => void;
}

/**
 * Expense detail modal with correction flow.
 *
 * Fetches expense data and correction history. Displays full details,
 * correction notices, and a timeline of corrections. The "Correct" button
 * opens an inline correction form pre-filled with the current values.
 */
export function ExpenseDetailModal({
  expenseId,
  currency,
  tags,
  currentYear,
  currentMonth,
  onClose,
  onCorrected,
}: ExpenseDetailModalProps) {
  const {
    expense,
    history,
    proRataGroup,
    viewState,
    error,
    setViewState,
    submitCorrection,
    correctionSubmitting,
    correctionError,
  } = useExpenseDetail(expenseId, { onCorrectionSuccess: onCorrected });

  return (
    <Dialog open={expenseId !== null} onOpenChange={() => onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {viewState === "correct" ? "Correct Expense" : "Expense Detail"}
          </DialogTitle>
          <DialogClose onClick={onClose} />
        </DialogHeader>

        {viewState === "loading" && (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
            <span className="ml-2 text-sm text-muted-foreground">
              Loading...
            </span>
          </div>
        )}

        {viewState === "error" && (
          <div className="flex items-center gap-2 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
            <AlertCircle className="size-4 shrink-0" />
            {error}
          </div>
        )}

        {viewState === "detail" && expense && (
          <DetailView
            expense={expense}
            currency={currency}
            tags={tags}
            history={history}
            proRataGroup={proRataGroup}
            currentYear={currentYear}
            currentMonth={currentMonth}
            onCorrectClick={() => setViewState("correct")}
          />
        )}

        {viewState === "correct" && expense && (
          <CorrectionForm
            expense={expense}
            currency={currency}
            tags={tags}
            onCancel={() => setViewState("detail")}
            onSubmit={submitCorrection}
            submitting={correctionSubmitting}
            submitError={correctionError}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
