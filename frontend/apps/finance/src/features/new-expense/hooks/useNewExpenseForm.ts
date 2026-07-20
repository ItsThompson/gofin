import { useState, useEffect, type SyntheticEvent } from "react";
import { apiClient, useFormMutation } from "@gofin/api";
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
}

export interface NewExpenseFormActions {
  setField: (key: keyof ExpenseFields, value: string) => void;
  clearFieldError: (field: string) => void;
  setIsProRata: (checked: boolean) => void;
  setProRataMonths: (value: string) => void;
  applySuggestion: (suggestion: ExpenseSuggestion) => void;
  handleSubmit: (event: SyntheticEvent<HTMLFormElement>) => void;
}

/**
 * Manages new expense form state: field values, validation,
 * tag fetching, and submission (including pro-rata).
 *
 * Composes useExpenseFields for field state and useFormMutation
 * for submission lifecycle. Total useState count: 4
 * (tags, tagsLoading, isProRata, proRataMonths).
 */
export function useNewExpenseForm(currency: string): {
  state: NewExpenseFormState;
  actions: NewExpenseFormActions;
} {
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const expenseFields = useExpenseFields();

  const [tags, setTags] = useState<Tag[]>([]);
  const [tagsLoading, setTagsLoading] = useState(true);
  const [isProRata, setIsProRata] = useState(false);
  const [proRataMonths, setProRataMonths] = useState("");

  const resetNewExpenseVisibleState = () => {
    expenseFields.reset({ tagId: getDefaultTagId(tags) });
    setIsProRata(false);
    setProRataMonths("");
  };

  const mutation = useFormMutation<SubmittedExpenseKind>({
    onSuccess: (submittedKind) => {
      toast.success(SUCCESS_TOAST_BY_KIND[submittedKind]);
      resetNewExpenseVisibleState();
    },
    onError: () => {
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
          totalAmount: amountCents,
          currency,
          expenseType: fields.expenseType,
          tagId: fields.tagId,
          expenseDate: fields.expenseDate,
          months: parseInt(proRataMonths, 10),
        };
        await apiClient<ProRataResponse>("/api/finance/prorata", {
          method: "POST",
          body: JSON.stringify(body),
        });
        return "proRata";
      }

      const body: CreateExpenseRequest = {
        name: fields.name.trim(),
        amount: amountCents,
        currency,
        expenseType: fields.expenseType,
        tagId: fields.tagId,
        expenseDate: fields.expenseDate,
        periodYear: currentYear,
        periodMonth: currentMonth,
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
    },
    actions: {
      setField: expenseFields.setField,
      clearFieldError: expenseFields.clearFieldError,
      setIsProRata: handleSetIsProRata,
      setProRataMonths,
      applySuggestion,
      handleSubmit,
    },
  };
}
