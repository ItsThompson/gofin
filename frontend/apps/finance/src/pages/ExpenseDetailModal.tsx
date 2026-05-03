import { useState, useEffect, useCallback, type FormEvent } from "react";
import {
  apiClient,
  ApiRequestError,
  formatCurrency,
  getCurrencySymbol,
  type Expense,
  type CorrectionHistoryResponse,
  type ExpenseResponse,
  type CorrectExpenseRequest,
  type Tag,
  type PaginatedResponse,
} from "@gofin/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
} from "@gofin/ui/components/dialog";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import {
  Loader2,
  AlertCircle,
  History,
  ArrowRight,
  Pencil,
} from "lucide-react";

/** Valid expense types matching the backend enum. */
const EXPENSE_TYPES = ["essentials", "desires", "savings"] as const;
type ExpenseType = (typeof EXPENSE_TYPES)[number];

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

type ModalView = "detail" | "correct";

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
  const [expense, setExpense] = useState<Expense | null>(null);
  const [history, setHistory] = useState<Expense[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<ModalView>("detail");
  const [proRataGroup, setProRataGroup] = useState<Expense[]>([]);

  const fetchExpenseData = useCallback(async () => {
    if (!expenseId) return;
    setLoading(true);
    setError(null);

    try {
      const [expenseResp, historyResp] = await Promise.all([
        apiClient<ExpenseResponse>(`/api/expenses/${expenseId}`),
        apiClient<CorrectionHistoryResponse>(
          `/api/expenses/${expenseId}/history`,
        ),
      ]);
      setExpense(expenseResp.expense);
      setHistory(historyResp.entries);

      // Fetch pro-rata group if this is a pro-rata expense
      if (expenseResp.expense.isProRata && expenseResp.expense.proRataGroup) {
        try {
          const groupResp = await apiClient<PaginatedResponse<Expense>>(
            `/api/expenses/prorata/${expenseResp.expense.proRataGroup}`,
          );
          setProRataGroup(groupResp.data);
        } catch {
          // Non-critical: pro-rata group display is supplementary
          setProRataGroup([]);
        }
      } else {
        setProRataGroup([]);
      }
    } catch {
      setError("Failed to load expense details.");
    } finally {
      setLoading(false);
    }
  }, [expenseId]);

  useEffect(() => {
    fetchExpenseData();
    setView("detail");
  }, [fetchExpenseData]);

  const tagMap = new Map(tags.map((tag) => [tag.id, tag.name]));

  const isCurrentPeriod =
    expense?.periodYear === currentYear &&
    expense?.periodMonth === currentMonth;
  const canCorrect = expense?.status === "active" && isCurrentPeriod;
  const hasCorrections = history.length > 1;

  // Find the correction that supersedes this expense (if corrected)
  const correctedBy =
    expense?.status === "corrected"
      ? history.find((entry) => entry.correctsId === expense.id)
      : null;

  // Find the expense this one corrects (if it's a correction)
  const correctsEntry = expense?.correctsId
    ? history.find((entry) => entry.id === expense.correctsId)
    : null;

  function handleCorrectionSuccess() {
    setView("detail");
    fetchExpenseData();
    onCorrected();
  }

  return (
    <Dialog open={expenseId !== null} onOpenChange={() => onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {view === "correct" ? "Correct Expense" : "Expense Detail"}
          </DialogTitle>
          <DialogClose onClick={onClose} />
        </DialogHeader>

        {loading && (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
            <span className="ml-2 text-sm text-muted-foreground">
              Loading...
            </span>
          </div>
        )}

        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
            <AlertCircle className="size-4 shrink-0" />
            {error}
          </div>
        )}

        {!loading && !error && expense && view === "detail" && (
          <DetailView
            expense={expense}
            currency={currency}
            tagMap={tagMap}
            canCorrect={canCorrect}
            correctedBy={correctedBy}
            correctsEntry={correctsEntry}
            hasCorrections={hasCorrections}
            history={history}
            proRataGroup={proRataGroup}
            onCorrectClick={() => setView("correct")}
          />
        )}

        {!loading && !error && expense && view === "correct" && (
          <CorrectionForm
            expense={expense}
            currency={currency}
            tags={tags}
            onCancel={() => setView("detail")}
            onSuccess={handleCorrectionSuccess}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

// --- Detail View ---

interface DetailViewProps {
  expense: Expense;
  currency: string;
  tagMap: Map<string, string>;
  canCorrect: boolean;
  correctedBy: Expense | null | undefined;
  correctsEntry: Expense | null | undefined;
  hasCorrections: boolean;
  history: Expense[];
  proRataGroup: Expense[];
  onCorrectClick: () => void;
}

function DetailView({
  expense,
  currency,
  tagMap,
  canCorrect,
  correctedBy,
  correctsEntry,
  hasCorrections,
  history,
  proRataGroup,
  onCorrectClick,
}: DetailViewProps) {
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
            tagMap={tagMap}
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

// --- Correction Timeline ---

interface CorrectionTimelineProps {
  entries: Expense[];
  currency: string;
  tagMap: Map<string, string>;
  currentExpenseId: string;
}

function CorrectionTimeline({
  entries,
  currency,
  tagMap,
  currentExpenseId,
}: CorrectionTimelineProps) {
  return (
    <div className="space-y-3">
      {entries.map((entry, index) => {
        const previous = index > 0 ? entries[index - 1] : null;
        const isCurrent = entry.id === currentExpenseId;
        const changes = previous ? computeChanges(previous, entry, currency, tagMap) : [];

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
                    key={change}
                    className="inline-flex items-center gap-1 rounded bg-blue-100 px-1.5 py-0.5 text-xs text-blue-800 dark:bg-blue-900/50 dark:text-blue-200"
                  >
                    <ArrowRight className="size-3" />
                    {change}
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

/**
 * Compare two entries and return a list of human-readable change descriptions.
 */
function computeChanges(
  previous: Expense,
  current: Expense,
  currency: string,
  tagMap: Map<string, string>,
): string[] {
  const changes: string[] = [];

  if (previous.name !== current.name) {
    changes.push(`Name: ${previous.name} → ${current.name}`);
  }
  if (previous.amount !== current.amount) {
    changes.push(
      `Amount: ${formatCurrency(previous.amount, currency)} → ${formatCurrency(current.amount, currency)}`,
    );
  }
  if (previous.expenseType !== current.expenseType) {
    changes.push(`Type: ${previous.expenseType} → ${current.expenseType}`);
  }
  if (previous.tagId !== current.tagId) {
    const prevTag = tagMap.get(previous.tagId) ?? previous.tagId;
    const currTag = tagMap.get(current.tagId) ?? current.tagId;
    changes.push(`Tag: ${prevTag} → ${currTag}`);
  }
  if (previous.expenseDate !== current.expenseDate) {
    changes.push(
      `Date: ${previous.expenseDate} → ${current.expenseDate}`,
    );
  }

  return changes;
}

// --- Correction Form ---

interface CorrectionFormProps {
  expense: Expense;
  currency: string;
  tags: Tag[];
  onCancel: () => void;
  onSuccess: () => void;
}

function CorrectionForm({
  expense,
  currency,
  tags,
  onCancel,
  onSuccess,
}: CorrectionFormProps) {
  const currencySymbol = getCurrencySymbol(currency);

  const [name, setName] = useState(expense.name);
  const [amountDollars, setAmountDollars] = useState(
    (expense.amount / 100).toFixed(2),
  );
  const [expenseType, setExpenseType] = useState<ExpenseType>(
    expense.expenseType,
  );
  const [tagId, setTagId] = useState(expense.tagId);
  const [expenseDate, setExpenseDate] = useState(expense.expenseDate);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function validate(): Record<string, string> {
    const errors: Record<string, string> = {};
    if (!name.trim()) errors.name = "Name is required";
    const parsed = parseFloat(amountDollars);
    if (!amountDollars || isNaN(parsed) || parsed <= 0) {
      errors.amount = "Amount must be greater than 0";
    }
    if (!expenseDate) errors.expenseDate = "Date is required";
    if (!tagId) errors.tagId = "Tag is required";
    return errors;
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setSubmitError(null);
    setFieldErrors({});

    const errors = validate();
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    const amountCents = Math.round(parseFloat(amountDollars) * 100);

    const body: CorrectExpenseRequest = {
      name: name.trim(),
      amount: amountCents,
      expenseType,
      tagId,
      expenseDate,
    };

    setSubmitting(true);
    try {
      await apiClient<ExpenseResponse>(
        `/api/expenses/${expense.id}/correct`,
        { method: "POST", body: JSON.stringify(body) },
      );
      onSuccess();
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.code === "ALREADY_CORRECTED") {
          setSubmitError(
            "This expense has already been corrected. Please close and refresh.",
          );
        } else if (err.code === "PERIOD_LOCKED") {
          setSubmitError(
            "Cannot correct expenses from a past period.",
          );
        } else {
          setSubmitError(err.message);
        }
      } else {
        setSubmitError("An unexpected error occurred.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Form onSubmit={handleSubmit}>
      {/* Name */}
      <FormField>
        <FormLabel htmlFor="correct-name">Name</FormLabel>
        <Input
          id="correct-name"
          type="text"
          value={name}
          onChange={(event) => {
            setName(event.target.value);
            setFieldErrors((prev) => ({ ...prev, name: "" }));
          }}
          aria-invalid={!!fieldErrors.name}
        />
        <FormMessage>{fieldErrors.name}</FormMessage>
      </FormField>

      {/* Amount */}
      <FormField>
        <FormLabel htmlFor="correct-amount">Amount</FormLabel>
        <div className="relative">
          <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
            {currencySymbol}
          </span>
          <Input
            id="correct-amount"
            type="number"
            min="0.01"
            step="0.01"
            value={amountDollars}
            onChange={(event) => {
              setAmountDollars(event.target.value);
              setFieldErrors((prev) => ({ ...prev, amount: "" }));
            }}
            className="pl-6"
            aria-invalid={!!fieldErrors.amount}
          />
        </div>
        <FormMessage>{fieldErrors.amount}</FormMessage>
      </FormField>

      {/* Expense Type */}
      <FormField>
        <FormLabel>Type</FormLabel>
        <div
          className="flex gap-4"
          role="radiogroup"
          aria-label="Expense type"
        >
          {EXPENSE_TYPES.map((type) => (
            <label
              key={type}
              className="flex cursor-pointer items-center gap-2"
            >
              <input
                type="radio"
                name="correctExpenseType"
                value={type}
                checked={expenseType === type}
                onChange={() => setExpenseType(type)}
                className="size-4 accent-primary"
              />
              <span className="text-sm capitalize">{type}</span>
            </label>
          ))}
        </div>
      </FormField>

      {/* Tag */}
      <FormField>
        <FormLabel htmlFor="correct-tag">Tag</FormLabel>
        <select
          id="correct-tag"
          value={tagId}
          onChange={(event) => {
            setTagId(event.target.value);
            setFieldErrors((prev) => ({ ...prev, tagId: "" }));
          }}
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          aria-invalid={!!fieldErrors.tagId}
        >
          {tags.map((tag) => (
            <option key={tag.id} value={tag.id}>
              {tag.name}
            </option>
          ))}
        </select>
        <FormMessage>{fieldErrors.tagId}</FormMessage>
      </FormField>

      {/* Date */}
      <FormField>
        <FormLabel htmlFor="correct-date">Date</FormLabel>
        <Input
          id="correct-date"
          type="date"
          value={expenseDate}
          onChange={(event) => {
            setExpenseDate(event.target.value);
            setFieldErrors((prev) => ({ ...prev, expenseDate: "" }));
          }}
          aria-invalid={!!fieldErrors.expenseDate}
        />
        <FormMessage>{fieldErrors.expenseDate}</FormMessage>
      </FormField>

      {/* Error */}
      {submitError && <FormMessage>{submitError}</FormMessage>}

      {/* Actions */}
      <div className="flex gap-2">
        <Button
          type="button"
          variant="outline"
          className="flex-1"
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button type="submit" className="flex-1" disabled={submitting}>
          {submitting ? "Saving..." : "Save Correction"}
        </Button>
      </div>
    </Form>
  );
}
