import type { Tag } from "../../types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
} from "@gofin/ui/components/dialog";
import { Loader2, AlertCircle } from "lucide-react";
import { useExpenseDetail } from "./hooks/useExpenseDetail";
import { useCorrectionForm } from "./hooks/useCorrectionForm";
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
  const state = useExpenseDetail(expenseId, {
    onCorrectionSuccess: onCorrected,
  });

  return (
    <Dialog open={expenseId !== null} onOpenChange={() => onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {state.status === "correct"
              ? "Correct Expense"
              : "Expense Detail"}
          </DialogTitle>
          <DialogClose onClick={onClose} />
        </DialogHeader>

        {state.status === "loading" && (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
            <span className="ml-2 text-sm text-muted-foreground">
              Loading...
            </span>
          </div>
        )}

        {state.status === "error" && (
          <div className="flex items-center gap-2 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
            <AlertCircle className="size-4 shrink-0" />
            {state.error}
          </div>
        )}

        {state.status === "detail" && (
          <DetailView
            expense={state.expense}
            currency={currency}
            tags={tags}
            history={state.history}
            proRataGroup={state.proRataGroup}
            currentYear={currentYear}
            currentMonth={currentMonth}
            onCorrectClick={state.startCorrection}
          />
        )}

        {state.status === "correct" && (
          <CorrectionFormContainer
            expense={state.expense}
            currency={currency}
            tags={tags}
            onCancel={state.cancelCorrection}
            onSubmit={state.correction.submitCorrection}
            submitting={state.correction.submitting}
            submitError={state.correction.error}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

/**
 * Container that instantiates useCorrectionForm and passes
 * state/actions to the presentational CorrectionForm component.
 */
function CorrectionFormContainer({
  expense,
  currency,
  tags,
  onCancel,
  onSubmit,
  submitting,
  submitError,
}: {
  expense: Parameters<typeof useCorrectionForm>[0];
  currency: string;
  tags: Tag[];
  onCancel: () => void;
  onSubmit: Parameters<typeof useCorrectionForm>[1];
  submitting: boolean;
  submitError: string | null;
}) {
  const { state, actions } = useCorrectionForm(expense, onSubmit, tags);

  return (
    <CorrectionForm
      currency={currency}
      tags={tags}
      fields={state.fields}
      fieldErrors={state.fieldErrors}
      submitting={submitting}
      submitError={submitError}
      onCancel={onCancel}
      onSubmit={actions.handleSubmit}
      onFieldChange={actions.setField}
      onSelectSuggestion={actions.applySuggestion}
    />
  );
}
