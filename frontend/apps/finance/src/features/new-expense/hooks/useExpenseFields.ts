import { useState, useCallback } from "react";
import { parseInput } from "@gofin/core";
import type { ExpenseFields, ValidateExpenseOptions } from "../../../lib/validate-expense-fields";
import { validateExpenseFields } from "../../../lib/validate-expense-fields";
import { toLocalISODate } from "../../../lib/date-utils";

/** Initial values for useExpenseFields. All fields optional: unset fields use defaults. */
export interface ExpenseFieldsInit {
  name?: string;
  amountDollars?: string;
  expenseType?: ExpenseFields["expenseType"];
  tagId?: string;
  expenseDateIso?: string;
}

/** Return type of useExpenseFields. */
export interface UseExpenseFieldsResult {
  /** Current field values. */
  fields: ExpenseFields;
  /** Current field-level validation errors. */
  fieldErrors: Record<string, string>;
  /** Update a single field by key. Clears that field's error. */
  setField: (key: keyof ExpenseFields, value: string) => void;
  /** Clear a specific field error (e.g., on focus). */
  clearFieldError: (field: string) => void;
  /** Run validation with given options. Updates fieldErrors. Returns true if valid. */
  validate: (options?: ValidateExpenseOptions) => boolean;
  /** Reset all fields to initial values. Clears all errors. */
  reset: (init?: ExpenseFieldsInit) => void;
  /** Derived: amount in selected currency minor units for payload construction. */
  amountCents: number;
}

function buildInitialFields(init?: ExpenseFieldsInit): ExpenseFields {
  return {
    name: init?.name ?? "",
    amountDollars: init?.amountDollars ?? "",
    expenseType: init?.expenseType ?? "essentials",
    tagId: init?.tagId ?? "",
    expenseDateIso: init?.expenseDateIso ?? toLocalISODate(),
  };
}

/**
 * Base hook for expense form field state management.
 *
 * Manages a single `fields` state object with `setField(key, value)` updater,
 * automatic field error clearing, validation delegation to `validateExpenseFields`,
 * and a derived `amountCents` value.
 *
 * Compose this with `useFormMutation` and feature-specific state to build
 * complete form hooks like `useNewExpenseForm` or `useCorrectionForm`.
 */
export function useExpenseFields(
  init?: ExpenseFieldsInit,
  currency = "USD",
): UseExpenseFieldsResult {
  const [fields, setFields] = useState<ExpenseFields>(() => buildInitialFields(init));
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const setField = useCallback((key: keyof ExpenseFields, value: string) => {
    setFields((prev) => ({ ...prev, [key]: value }));

    // Map field key to error key (amountDollars → amount for consistency)
    const errorKey = key === "amountDollars" ? "amount" : key;
    setFieldErrors((prev) => {
      if (!prev[errorKey]) return prev;
      const next = { ...prev };
      delete next[errorKey];
      return next;
    });
  }, []);

  const clearFieldError = useCallback((field: string) => {
    setFieldErrors((prev) => {
      if (!prev[field]) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
  }, []);

  const validate = useCallback(
    (options?: ValidateExpenseOptions): boolean => {
      const errors = validateExpenseFields(fields, { currency, ...options });
      setFieldErrors(errors);
      return Object.keys(errors).length === 0;
    },
    [fields, currency],
  );

  const reset = useCallback((resetInit?: ExpenseFieldsInit) => {
    setFields(buildInitialFields(resetInit));
    setFieldErrors({});
  }, []);

  let amountCents = 0;
  try {
    amountCents = parseInput(fields.amountDollars, currency);
  } catch {
    amountCents = 0;
  }

  return {
    fields,
    fieldErrors,
    setField,
    clearFieldError,
    validate,
    reset,
    amountCents,
  };
}
