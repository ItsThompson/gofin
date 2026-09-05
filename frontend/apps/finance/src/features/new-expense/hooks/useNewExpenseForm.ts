import { useState, useEffect, useRef, type SyntheticEvent } from "react";
import { ApiRequestError, apiClient, useFormMutation } from "@gofin/api";
import { toast } from "sonner";
import { EXPENSE_TYPES, type ExpenseType } from "@gofin/core";
import type { ExpenseFields } from "../../../lib/validate-expense-fields";
import type {
  ExpenseResponse,
  CreateExpenseRequest,
  CreateProRataRequest,
  ProRataResponse,
  Tag,
  TagListResponse,
} from "@gofin/core";
import {
  createExpenseSuggestionPatch,
  type ExpenseSuggestion,
} from "../../expense-autocomplete";
import type { SubmittedExpenseKind } from "../types";
import { useExpenseFields } from "./useExpenseFields";

export { EXPENSE_TYPES };
export type { ExpenseType, ExpenseFields };

const SUCCESS_TOAST_BY_KIND: Record<SubmittedExpenseKind, string> = {
  standard: "Expense saved",
  proRata: "Expense schedule saved",
};

function getDefaultTagId(tags: Tag[]): string {
  return tags[0]?.id ?? "";
}

export interface NewExpenseFormState {
  tags: Tag[];
  tagsLoading: boolean;
  fields: ExpenseFields;
  fieldErrors: Record<string, string>;
  isProRata: boolean;
  proRataMonths: string;
  error: string | null;
  submitting: boolean;
  transactionCurrencyCode: string;
}

export interface NewExpenseFormActions {
  setField: (key: keyof ExpenseFields, value: string) => void;
  clearFieldError: (field: string) => void;
  setIsProRata: (checked: boolean) => void;
  setProRataMonths: (value: string) => void;
  setTransactionCurrency: (value: string) => void;
  applySuggestion: (suggestion: ExpenseSuggestion) => void;
  handleSubmit: (event: SyntheticEvent<HTMLFormElement>) => void;
}

/**
 * Manages new expense form state: field values, validation,
 * tag fetching, and submission (including pro-rata). Composes
 * useExpenseFields for field state and useFormMutation for the
 * submission lifecycle.
 */
export function useNewExpenseForm(
  currency: string,
  periodYear: number,
  periodMonth: number,
): {
  state: NewExpenseFormState;
  actions: NewExpenseFormActions;
} {
  const [transactionCurrencyCode, setTransactionCurrency] = useState(currency);
  const expenseFields = useExpenseFields(undefined, transactionCurrencyCode);

  // One idempotency key per logical submit. Generated once on mount and reused
  // across retries of the same submit; reset to a fresh UUID only after a
  // successful save so a network retry returns the already-created expense.
  const idempotencyKeyRef = useRef(crypto.randomUUID());

  useEffect(() => {
    setTransactionCurrency(currency);
  }, [currency]);

  const [tags, setTags] = useState<Tag[]>([]);
  const [tagsLoading, setTagsLoading] = useState(true);
  const [isProRata, setIsProRata] = useState(false);
  const [proRataMonths, setProRataMonths] = useState("");

  const resetNewExpenseVisibleState = () => {
    expenseFields.reset({ tagId: getDefaultTagId(tags) });
    setIsProRata(false);
    setProRataMonths("");
    idempotencyKeyRef.current = crypto.randomUUID();
  };

  const mutation = useFormMutation<SubmittedExpenseKind>({
    onSuccess: (submittedKind) => {
      toast.success(SUCCESS_TOAST_BY_KIND[submittedKind]);
      resetNewExpenseVisibleState();
    },
    onError: (message, cause) => {
      // FX conversion-unavailable failures show the spec-mandated guidance
      // toast instead of the generic failure message. The form error banner
      // (state.error) already carries the server's guidance copy. Form values
      // are preserved so the user can retry or manually convert.
      if (cause instanceof ApiRequestError && cause.code === "CONVERSION_UNAVAILABLE") {
        toast.error(message);
        return;
      }
      toast.error("Failed to save expense");
    },
  });

  useEffect(() => {
    async function fetchTags() {
      try {
        const response = await apiClient<TagListResponse>("/api/finance/tags");
        setTags(response.tags);
        const defaultTagId = getDefaultTagId(response.tags);
        if (defaultTagId) {
          expenseFields.setField("tagId", defaultTagId);
        }
      } catch {
        // Tags fail silently: form will show empty dropdown
      } finally {
        setTagsLoading(false);
      }
    }
    fetchTags();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function handleSetIsProRata(checked: boolean) {
    setIsProRata(checked);
    if (!checked) {
      setProRataMonths("");
      expenseFields.clearFieldError("proRataMonths");
    }
  }

  function applySuggestion(suggestion: ExpenseSuggestion) {
    const patch = createExpenseSuggestionPatch(suggestion, tags);

    expenseFields.setField("name", patch.name);
    expenseFields.setField("amountDollars", patch.amountDollars);
    expenseFields.setField("expenseType", patch.expenseType);

    setTransactionCurrency(patch.currency);

    if (patch.tagId) {
      expenseFields.setField("tagId", patch.tagId);
    }
  }

  function handleSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();

    const isValid = expenseFields.validate({
      isProRata,
      proRataMonths,
    });
    if (!isValid) return;

    const { fields, amountCents } = expenseFields;

    mutation.submit(async () => {
      if (isProRata) {
        const body: CreateProRataRequest = {
          name: fields.name.trim(),
          totalAmountInMinorUnits: amountCents,
          transactionCurrencyCode,
          expenseType: fields.expenseType,
          tagId: fields.tagId,
          expenseDateIso: fields.expenseDateIso,
          periodYear,
          periodMonth,
          spreadOverMonths: parseInt(proRataMonths, 10),
        };
        await apiClient<ProRataResponse>("/api/finance/prorata", {
          method: "POST",
          body: JSON.stringify(body),
        });
        return "proRata";
      }

      const body: CreateExpenseRequest = {
        name: fields.name.trim(),
        amountInTransactionCurrencyMinorUnits: amountCents,
        transactionCurrencyCode,
        expenseType: fields.expenseType,
        tagId: fields.tagId,
        expenseDateIso: fields.expenseDateIso,
        periodYear,
        periodMonth,
        clientGeneratedIdempotencyKey: idempotencyKeyRef.current,
      };
      await apiClient<ExpenseResponse>("/api/expenses", {
        method: "POST",
        body: JSON.stringify(body),
      });
      return "standard";
    });
  }

  return {
    state: {
      tags,
      tagsLoading,
      fields: expenseFields.fields,
      fieldErrors: expenseFields.fieldErrors,
      isProRata,
      proRataMonths,
      error: mutation.error,
      submitting: mutation.submitting,
      transactionCurrencyCode,
    },
    actions: {
      setField: expenseFields.setField,
      clearFieldError: expenseFields.clearFieldError,
      setIsProRata: handleSetIsProRata,
      setProRataMonths,
      setTransactionCurrency,
      applySuggestion,
      handleSubmit,
    },
  };
}
