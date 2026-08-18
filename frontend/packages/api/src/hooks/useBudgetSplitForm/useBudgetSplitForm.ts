import { useState, useCallback } from "react";
import {
  validateEDSSplit,
  hasValidMinorUnitPrecision,
  toMinorUnits,
} from "@gofin/core";
import type {
  BudgetSplitFields,
  BudgetSplitForm,
  BudgetSplitFormOptions,
} from "./types";
import {
  buildInitialFields,
  parseSplitPercentages,
  precisionError,
} from "./utils";

export function useBudgetSplitForm(options?: BudgetSplitFormOptions): BudgetSplitForm {
  const currency = options?.currency ?? "USD";
  const [fields, setFields] = useState<BudgetSplitFields>(() =>
    buildInitialFields(options),
  );

  const setField = useCallback(
    (field: keyof BudgetSplitFields, value: string) => {
      setFields((prev) => ({ ...prev, [field]: value }));
    },
    [],
  );

  const { essentials: essentialsNum, desires: desiresNum, savings: savingsNum } =
    parseSplitPercentages(fields);

  const splitTotal = essentialsNum + desiresNum + savingsNum;
  const splitError = validateEDSSplit(essentialsNum, desiresNum, savingsNum);

  const validate = useCallback((): string | null => {
    const { essentials, desires, savings } = parseSplitPercentages(fields);

    const edsSplitError = validateEDSSplit(essentials, desires, savings);
    if (edsSplitError) return edsSplitError;

    const budgetValue = parseFloat(fields.budgetDollars) || 0;
    if (budgetValue < 0) {
      return "Budget amount must be non-negative";
    }

    if (!hasValidMinorUnitPrecision(fields.budgetDollars, currency)) {
      return precisionError(currency);
    }

    return null;
  }, [fields, currency]);

  const toPayload = useCallback(() => {
    const { essentials, desires, savings } = parseSplitPercentages(fields);

    return {
      budgetAmountCents: toMinorUnits(fields.budgetDollars, currency),
      essentialsPercent: essentials,
      desiresPercent: desires,
      savingsPercent: savings,
    };
  }, [fields, currency]);

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
