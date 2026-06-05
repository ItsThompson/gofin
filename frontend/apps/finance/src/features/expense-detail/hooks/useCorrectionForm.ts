import { useCallback, type FormEvent } from "react";
import type { ExpenseFields } from "../../../lib/validate-expense-fields";
import type { Expense, CorrectExpenseRequest, Tag } from "../../../types";
import {
  createExpenseSuggestionPatch,
  type ExpenseSuggestion,
} from "../../expense-autocomplete";
import { useExpenseFields } from "../../new-expense/hooks/useExpenseFields";

/** State returned by useCorrectionForm (field-level only). */
export interface CorrectionFormState {
  /** Expense field values (pre-filled from existing expense). */
  fields: ExpenseFields;
  /** Field-level validation errors. */
  fieldErrors: Record<string, string>;
}

/** Actions returned by useCorrectionForm. */
export interface CorrectionFormActions {
  /** Update an expense field. */
  setField: (key: keyof ExpenseFields, value: string) => void;
  /** Clear a field error. */
  clearFieldError: (field: string) => void;
  /** Apply fields from an explicitly selected historical suggestion. */
  applySuggestion: (suggestion: ExpenseSuggestion) => void;
  /** Submit the correction (validates then calls onSubmit). */
  handleSubmit: (event: FormEvent) => void;
}

/**
 * Manages correction form field state by composing useExpenseFields
 * with initial values derived from the existing expense.
 *
 * Submission lifecycle (submitting/error) is managed by the parent's
 * useFormMutation: this hook only validates and builds the request body,
 * then delegates to the provided onSubmit callback.
 */
export function useCorrectionForm(
  expense: Expense,
  onSubmit: (form: CorrectExpenseRequest) => void,
  tags: Tag[] = [],
): { state: CorrectionFormState; actions: CorrectionFormActions } {
  const expenseFields = useExpenseFields({
    name: expense.name,
    amountDollars: (expense.amount / 100).toFixed(2),
    expenseType: expense.expenseType,
    tagId: expense.tagId,
    expenseDate: expense.expenseDate,
  });

  const applySuggestion = useCallback(
    (suggestion: ExpenseSuggestion) => {
      const patch = createExpenseSuggestionPatch(suggestion, tags);

      expenseFields.setField("name", patch.name);
      expenseFields.setField("amountDollars", patch.amountDollars);
      expenseFields.setField("expenseType", patch.expenseType);

      if (patch.tagId) {
        expenseFields.setField("tagId", patch.tagId);
      }
    },
    [expenseFields, tags],
  );

  const handleSubmit = useCallback(
    (event: FormEvent) => {
      event.preventDefault();

      // No pro-rata validation for corrections
      const isValid = expenseFields.validate();
      if (!isValid) return;

      const { fields, amountCents } = expenseFields;

      const body: CorrectExpenseRequest = {
        name: fields.name.trim(),
        amount: amountCents,
        expenseType: fields.expenseType,
        tagId: fields.tagId,
        expenseDate: fields.expenseDate,
      };

      onSubmit(body);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [expenseFields.validate, expenseFields.fields, expenseFields.amountCents, onSubmit],
  );

  return {
    state: {
      fields: expenseFields.fields,
      fieldErrors: expenseFields.fieldErrors,
    },
    actions: {
      setField: expenseFields.setField,
      clearFieldError: expenseFields.clearFieldError,
      applySuggestion,
      handleSubmit,
    },
  };
}
