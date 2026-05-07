import { useState, useCallback } from "react";
import { validateEDSSplit, DEFAULT_BUDGET_SPLIT, toCents } from "@gofin/core";

/** Options for initializing the budget split form. */
export interface BudgetSplitFormOptions {
  /** Initial budget amount in cents. Converted to dollars for display. */
  initialBudgetCents?: number;
  /** Initial E/D/S percentages. Defaults to DEFAULT_BUDGET_SPLIT if not provided. */
  initialSplit?: { essentials: number; desires: number; savings: number };
}

/** Field values as strings (for controlled inputs). */
export interface BudgetSplitFields {
  budgetDollars: string;
  essentials: string;
  desires: string;
  savings: string;
}

/** Return type of useBudgetSplitForm. */
export interface BudgetSplitForm {
  /** Current field values (strings for input binding). */
  fields: BudgetSplitFields;
  /** Update a single field by name. */
  setField: (field: keyof BudgetSplitFields, value: string) => void;
  /** Current sum of E+D+S percentages (derived, not stored). */
  splitTotal: number;
  /** Current validation error, or null if valid. Updated on every render. */
  splitError: string | null;
  /** Explicitly validate all fields. Returns error string or null. */
  validate: () => string | null;
  /** Convert current field values to the API request payload. */
  toPayload: () => {
    budgetAmountCents: number;
    essentialsPercent: number;
    desiresPercent: number;
    savingsPercent: number;
  };
  /** Reset form to given options (or defaults). */
  reset: (options?: BudgetSplitFormOptions) => void;
}

function buildInitialFields(options?: BudgetSplitFormOptions): BudgetSplitFields {
  const split = options?.initialSplit ?? DEFAULT_BUDGET_SPLIT;
  let budgetDollars = "";
  if (options?.initialBudgetCents != null) {
    budgetDollars = (options.initialBudgetCents / 100).toString();
  }

  return {
    budgetDollars,
    essentials: String(split.essentials),
    desires: String(split.desires),
    savings: String(split.savings),
  };
}

/**
 * Shared hook for budget split form state, validation, and payload construction.
 *
 * Consolidates duplicated form logic from CreatePeriodPrompt, BudgetSettingsEditor,
 * DefaultBudgetSection, and SplitStep. Includes the non-negative validation check
 * that was previously missing in the settings page.
 */
export function useBudgetSplitForm(options?: BudgetSplitFormOptions): BudgetSplitForm {
  const [fields, setFields] = useState<BudgetSplitFields>(() =>
    buildInitialFields(options),
  );

  const setField = useCallback(
    (field: keyof BudgetSplitFields, value: string) => {
      setFields((prev) => ({ ...prev, [field]: value }));
    },
    [],
  );

  // Derived values: computed each render, not stored in state.
  const essentialsNum = parseInt(fields.essentials, 10) || 0;
  const desiresNum = parseInt(fields.desires, 10) || 0;
  const savingsNum = parseInt(fields.savings, 10) || 0;

  const splitTotal = essentialsNum + desiresNum + savingsNum;
  const splitError = validateEDSSplit(essentialsNum, desiresNum, savingsNum);

  const validate = useCallback((): string | null => {
    const e = parseInt(fields.essentials, 10) || 0;
    const d = parseInt(fields.desires, 10) || 0;
    const s = parseInt(fields.savings, 10) || 0;

    // Non-negative check for percentages (delegated to validateEDSSplit)
    const edsSplitError = validateEDSSplit(e, d, s);
    if (edsSplitError) return edsSplitError;

    // Budget non-negative check
    const budgetValue = parseFloat(fields.budgetDollars) || 0;
    if (budgetValue < 0) {
      return "Budget amount must be non-negative";
    }

    return null;
  }, [fields]);

  const toPayload = useCallback(() => {
    return {
      budgetAmountCents: toCents(fields.budgetDollars),
      essentialsPercent: parseInt(fields.essentials, 10) || 0,
      desiresPercent: parseInt(fields.desires, 10) || 0,
      savingsPercent: parseInt(fields.savings, 10) || 0,
    };
  }, [fields]);

  const reset = useCallback((resetOptions?: BudgetSplitFormOptions) => {
    setFields(buildInitialFields(resetOptions));
  }, []);

  return {
    fields,
    setField,
    splitTotal,
    splitError,
    validate,
    toPayload,
    reset,
  };
}
