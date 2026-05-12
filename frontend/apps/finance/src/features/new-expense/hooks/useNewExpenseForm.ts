import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { apiClient, useFormMutation } from "@gofin/api";
import { EXPENSE_TYPES, type ExpenseType } from "@gofin/core";
import type { ExpenseFields } from "../../../lib/validate-expense-fields";
import type {
  ExpenseResponse,
  CreateExpenseRequest,
  CreateProRataRequest,
  ProRataResponse,
  Tag,
  TagListResponse,
} from "../../../types";
import { useExpenseFields } from "./useExpenseFields";

export { EXPENSE_TYPES };
export type { ExpenseType, ExpenseFields };

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
  handleSubmit: (event: FormEvent) => void;
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
  const navigate = useNavigate();
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  // Field state management via base hook
  const expenseFields = useExpenseFields();

  // Feature-specific state (4 useState calls)
  const [tags, setTags] = useState<Tag[]>([]);
  const [tagsLoading, setTagsLoading] = useState(true);
  const [isProRata, setIsProRata] = useState(false);
  const [proRataMonths, setProRataMonths] = useState("");

  // Submission lifecycle via useFormMutation
  const mutation = useFormMutation<void>({
    onSuccess: () => navigate("/dashboard"),
  });

  useEffect(() => {
    async function fetchTags() {
      try {
        const response = await apiClient<TagListResponse>("/api/finance/tags");
        setTags(response.tags);
        if (response.tags.length > 0) {
          expenseFields.setField("tagId", response.tags[0].id);
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

  function handleSubmit(event: FormEvent) {
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
      } else {
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
      }
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
      handleSubmit,
    },
  };
}
