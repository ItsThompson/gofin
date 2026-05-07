import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { apiClient, ApiRequestError } from "@gofin/api";
import { EXPENSE_TYPES, type ExpenseType } from "@gofin/core";
import type {
  ExpenseResponse,
  CreateExpenseRequest,
  CreateProRataRequest,
  ProRataResponse,
  Tag,
  TagListResponse,
} from "../../../types";

function todayISO(): string {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export { EXPENSE_TYPES };
export type { ExpenseType };

export interface NewExpenseFormState {
  tags: Tag[];
  tagsLoading: boolean;
  name: string;
  amountDollars: string;
  expenseType: ExpenseType;
  tagId: string;
  expenseDate: string;
  error: string | null;
  fieldErrors: Record<string, string>;
  submitting: boolean;
  isProRata: boolean;
  proRataMonths: string;
}

export interface NewExpenseFormActions {
  setName: (value: string) => void;
  setAmountDollars: (value: string) => void;
  setExpenseType: (type: ExpenseType) => void;
  setTagId: (id: string) => void;
  setExpenseDate: (date: string) => void;
  setIsProRata: (checked: boolean) => void;
  setProRataMonths: (value: string) => void;
  clearFieldError: (field: string) => void;
  handleSubmit: (event: FormEvent) => void;
}

/**
 * Manages new expense form state: field values, validation,
 * tag fetching, and submission (including pro-rata).
 */
export function useNewExpenseForm(currency: string): {
  state: NewExpenseFormState;
  actions: NewExpenseFormActions;
} {
  const navigate = useNavigate();
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const [tags, setTags] = useState<Tag[]>([]);
  const [tagsLoading, setTagsLoading] = useState(true);
  const [name, setName] = useState("");
  const [amountDollars, setAmountDollars] = useState("");
  const [expenseType, setExpenseType] = useState<ExpenseType>("essentials");
  const [tagId, setTagId] = useState("");
  const [expenseDate, setExpenseDate] = useState(todayISO());
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [isProRata, setIsProRata] = useState(false);
  const [proRataMonths, setProRataMonths] = useState("");

  useEffect(() => {
    async function fetchTags() {
      try {
        const response = await apiClient<TagListResponse>("/api/finance/tags");
        setTags(response.tags);
        if (response.tags.length > 0) {
          setTagId(response.tags[0].id);
        }
      } catch {
        // Tags fail silently: form will show empty dropdown
      } finally {
        setTagsLoading(false);
      }
    }
    fetchTags();
  }, []);

  function validate(): Record<string, string> {
    const errors: Record<string, string> = {};

    if (!name.trim()) {
      errors.name = "Name is required";
    }

    const parsedAmount = parseFloat(amountDollars);
    if (!amountDollars || isNaN(parsedAmount) || parsedAmount <= 0) {
      errors.amount = "Amount must be greater than 0";
    }

    if (!expenseDate) {
      errors.expenseDate = "Date is required";
    }

    if (!tagId) {
      errors.tagId = "Tag is required";
    }

    if (isProRata) {
      const months = parseInt(proRataMonths, 10);
      if (!proRataMonths || isNaN(months) || months < 2) {
        errors.proRataMonths = "Must be at least 2 months";
      }
    }

    return errors;
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});

    const errors = validate();
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    const amountCents = Math.round(parseFloat(amountDollars) * 100);

    setSubmitting(true);
    try {
      if (isProRata) {
        const body: CreateProRataRequest = {
          name: name.trim(),
          totalAmount: amountCents,
          currency,
          expenseType,
          tagId,
          expenseDate,
          months: parseInt(proRataMonths, 10),
        };
        await apiClient<ProRataResponse>("/api/finance/prorata", {
          method: "POST",
          body: JSON.stringify(body),
        });
      } else {
        const body: CreateExpenseRequest = {
          name: name.trim(),
          amount: amountCents,
          currency,
          expenseType,
          tagId,
          expenseDate,
          periodYear: currentYear,
          periodMonth: currentMonth,
        };
        await apiClient<ExpenseResponse>("/api/expenses", {
          method: "POST",
          body: JSON.stringify(body),
        });
      }
      navigate("/dashboard");
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred. Please try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  function clearFieldError(field: string) {
    setFieldErrors((prev) => ({ ...prev, [field]: "" }));
  }

  function handleSetIsProRata(checked: boolean) {
    setIsProRata(checked);
    if (!checked) {
      setProRataMonths("");
      setFieldErrors((prev) => ({ ...prev, proRataMonths: "" }));
    }
  }

  return {
    state: {
      tags,
      tagsLoading,
      name,
      amountDollars,
      expenseType,
      tagId,
      expenseDate,
      error,
      fieldErrors,
      submitting,
      isProRata,
      proRataMonths,
    },
    actions: {
      setName,
      setAmountDollars,
      setExpenseType,
      setTagId,
      setExpenseDate,
      setIsProRata: handleSetIsProRata,
      setProRataMonths,
      clearFieldError,
      handleSubmit,
    },
  };
}
