import { useCallback, useState, type FormEvent } from "react";
import type { ExpenseFields } from "../../../lib/validate-expense-fields";
import { getMinorUnitDigits, toMajorUnits } from "@gofin/core";
import type { Expense, CorrectExpenseRequest, Tag } from "@gofin/core";
import {
  createExpenseSuggestionPatch,
  type ExpenseSuggestion,
} from "../../expense-autocomplete";
import { useExpenseFields } from "../../new-expense/hooks/useExpenseFields";

/** State returned by useCorrectionForm (field-level only). */
export interface CorrectionFormState {
  /** Expense field values pre-filled from the active expense. */
  fields: ExpenseFields;
  /** Selected transaction currency for the correction. */
  transactionCurrencyCode: string;
  /** Field-level validation errors. */
  fieldErrors: Record<string, string>;
}

/** Actions returned by useCorrectionForm. */
export interface CorrectionFormActions {
  /** Update an expense field. */
  setField: (key: keyof ExpenseFields, value: string) => void;
  /** Clear a field error. */
  clearFieldError: (field: string) => void;
  /** Update the transaction currency and revalidate the amount. */
  setTransactionCurrency: (value: string) => void;
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
  const originalTransactionAmountInMinorUnits = expense.originalTransactionAmountInMinorUnits;
  const initialTransactionCurrency = expense.transactionCurrencyCode;
  const [transactionCurrencyCode, setTransactionCurrencyState] = useState(initialTransactionCurrency);
  const expenseFields = useExpenseFields(
    {
      name: expense.name,
      amountDollars: toMajorUnits(originalTransactionAmountInMinorUnits, initialTransactionCurrency).toFixed(
        getMinorUnitDigits(initialTransactionCurrency),
      ),
      expenseType: expense.expenseType,
      tagId: expense.tagId,
      expenseDateIso: expense.expenseDateIso,
    },
    transactionCurrencyCode,
  );

  const applySuggestion = useCallback(
    (suggestion: ExpenseSuggestion) => {
      const patch = createExpenseSuggestionPatch(suggestion, tags);

      expenseFields.setField("name", patch.name);
      expenseFields.setField("amountDollars", patch.amountDollars);
      expenseFields.setField("expenseType", patch.expenseType);
      setTransactionCurrencyState(patch.currency);

      if (patch.tagId) {
        expenseFields.setField("tagId", patch.tagId);
      }
    },
    [expenseFields, tags],
  );

  const setTransactionCurrency = useCallback(
    (value: string) => {
      setTransactionCurrencyState(value);
      if (expenseFields.fields.amountDollars) {
        const isValid = expenseFields.validate({ currency: value });
        if (isValid) {
          expenseFields.clearFieldError("amount");
        }
      }
    },
    [expenseFields],
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
        amountInTransactionCurrencyMinorUnits: amountCents,
        expenseType: fields.expenseType,
        tagId: fields.tagId,
        expenseDateIso: fields.expenseDateIso,
        transactionCurrencyCode,
      };

      onSubmit(body);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      expenseFields.validate,
      expenseFields.fields,
      expenseFields.amountCents,
      onSubmit,
      transactionCurrencyCode,
    ],
  );

  return {
    state: {
      fields: expenseFields.fields,
      transactionCurrencyCode,
      fieldErrors: expenseFields.fieldErrors,
    },
    actions: {
      setField: expenseFields.setField,
      clearFieldError: expenseFields.clearFieldError,
      setTransactionCurrency,
      applySuggestion,
      handleSubmit,
    },
  };
}
