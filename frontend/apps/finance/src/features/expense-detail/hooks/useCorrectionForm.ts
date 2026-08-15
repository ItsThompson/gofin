import { useCallback, useState, type FormEvent } from "react";
import type { ExpenseFields } from "../../../lib/validate-expense-fields";
import { getMinorUnitDigits, toMajorUnits } from "@gofin/core";
import type { Expense, CorrectExpenseRequest, Tag } from "@gofin/core";
import {
  createExpenseSuggestionPatch,
  type ExpenseSuggestion,
} from "../../expense-autocomplete";
import { useExpenseFields } from "../../new-expense/hooks/useExpenseFields";
import { getTransactionAmount, getTransactionCurrency } from "../utils/moneyFacts";

/** State returned by useCorrectionForm (field-level only). */
export interface CorrectionFormState {
  /** Expense field values pre-filled from the active expense. */
  fields: ExpenseFields;
  /** Selected transaction currency for the correction. */
  transactionCurrency: string;
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
  const transactionAmount = getTransactionAmount(expense);
  const initialTransactionCurrency = getTransactionCurrency(expense);
  const [transactionCurrency, setTransactionCurrencyState] = useState(initialTransactionCurrency);
  const expenseFields = useExpenseFields(
    {
      name: expense.name,
      amountDollars: toMajorUnits(transactionAmount, initialTransactionCurrency).toFixed(
        getMinorUnitDigits(initialTransactionCurrency),
      ),
      expenseType: expense.expenseType,
      tagId: expense.tagId,
      expenseDate: expense.expenseDate,
    },
    transactionCurrency,
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
        amount: amountCents,
        expenseType: fields.expenseType,
        tagId: fields.tagId,
        expenseDate: fields.expenseDate,
        transactionCurrency,
      };

      onSubmit(body);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      expenseFields.validate,
      expenseFields.fields,
      expenseFields.amountCents,
      onSubmit,
      transactionCurrency,
    ],
  );

  return {
    state: {
      fields: expenseFields.fields,
      transactionCurrency,
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
