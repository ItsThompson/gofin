import type { Expense, CorrectExpenseRequest } from "@gofin/core";

/** Correction-related state, available only when expense data is loaded. */
export interface CorrectionState {
  submitCorrection: (form: CorrectExpenseRequest) => void;
  submitting: boolean;
  error: string | null;
  clearError: () => void;
}

/** Loading expense data. */
export interface ExpenseDetailLoading {
  status: "loading";
  refresh: () => void;
}

/** Expense loaded, viewing details. */
export interface ExpenseDetailView {
  status: "detail";
  expense: Expense;
  history: Expense[];
  proRataGroup: Expense[];
  correction: CorrectionState;
  /** Transition to correction mode. */
  startCorrection: () => void;
  refresh: () => void;
}

/** Expense loaded, correction form active. */
export interface ExpenseDetailCorrect {
  status: "correct";
  expense: Expense;
  history: Expense[];
  proRataGroup: Expense[];
  correction: CorrectionState;
  /** Return to detail view. */
  cancelCorrection: () => void;
  refresh: () => void;
}

/** Fetch failed. */
export interface ExpenseDetailError {
  status: "error";
  error: string;
  refresh: () => void;
}

export type ExpenseDetailState =
  | ExpenseDetailLoading
  | ExpenseDetailView
  | ExpenseDetailCorrect
  | ExpenseDetailError;

export interface UseExpenseDetailOptions {
  /** Called after a correction is successfully submitted. */
  onCorrectionSuccess?: () => void;
}
