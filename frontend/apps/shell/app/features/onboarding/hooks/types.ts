import type { FormEvent } from "react";

export type OnboardingStep = "welcome" | "currency" | "budget" | "split";

export interface SplitForm {
  essentials: string;
  desires: string;
  savings: string;
  setEssentials: (value: string) => void;
  setDesires: (value: string) => void;
  setSavings: (value: string) => void;
  splitError: string | null;
  clearSplitError: () => void;
}

/** Read-only values describing current onboarding state. */
export interface OnboardingState {
  /** Current active step in the onboarding wizard. */
  currentStep: OnboardingStep;
  /** Zero-based index of the current step. */
  stepIndex: number;
  /** Total number of steps. */
  totalSteps: number;
  /** Selected currency code. */
  currency: string;
  /** Budget amount as a dollar string (for controlled input). */
  budgetDollars: string;
  /** Budget split form state. */
  splitForm: SplitForm;
  /** Whether submission is in progress. */
  submitting: boolean;
  /** Submission error message, or null. */
  error: string | null;
}

/** Callable functions that mutate onboarding state or trigger side effects. */
export interface OnboardingActions {
  /** Set the currency code. */
  setCurrency: (code: string) => void;
  /** Set the budget dollar amount. */
  setBudgetDollars: (value: string) => void;
  /** Advance to the next step. */
  goNext: () => void;
  /** Return to the previous step. */
  goBack: () => void;
  /** Skip the current step (applies defaults). */
  skipStep: () => void;
  /** Submit the onboarding form. */
  submit: (event?: FormEvent) => void;
}
