export interface BudgetSplitFormOptions {
  initialBudgetCents?: number;
  currency?: string;
  /** Defaults to DEFAULT_BUDGET_SPLIT when omitted. */
  initialSplit?: { essentials: number; desires: number; savings: number };
}

/** Values are strings so they bind directly to controlled inputs. */
export interface BudgetSplitFields {
  budgetDollars: string;
  essentials: string;
  desires: string;
  savings: string;
}

export interface BudgetSplitPayload {
  budgetAmountCents: number;
  essentialsPercent: number;
  desiresPercent: number;
  savingsPercent: number;
}

export interface BudgetSplitForm {
  fields: BudgetSplitFields;
  setField: (field: keyof BudgetSplitFields, value: string) => void;
  splitTotal: number;
  /** Derived each render from the E/D/S fields only; validate() also checks the budget amount. */
  splitError: string | null;
  validate: () => string | null;
  toPayload: () => BudgetSplitPayload;
  reset: (options?: BudgetSplitFormOptions) => void;
}
