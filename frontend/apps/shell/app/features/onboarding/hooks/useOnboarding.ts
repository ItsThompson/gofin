import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { apiClient, ApiRequestError } from "@gofin/api";

export type OnboardingStep = "welcome" | "currency" | "budget" | "split";

const STEP_ORDER: OnboardingStep[] = ["welcome", "currency", "budget", "split"];

/** Default values applied when a step is skipped. */
const DEFAULTS = {
  currency: "USD",
  budgetDollars: 0,
  essentials: 50,
  desires: 30,
  savings: 20,
};

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

export interface OnboardingResult {
  currentStep: OnboardingStep;
  stepIndex: number;
  totalSteps: number;
  currency: string;
  setCurrency: (code: string) => void;
  budgetDollars: string;
  setBudgetDollars: (value: string) => void;
  splitForm: SplitForm;
  goNext: () => void;
  goBack: () => void;
  skipStep: () => void;
  submit: (event?: FormEvent) => void;
  submitting: boolean;
  error: string | null;
}

/** Validates E/D/S split sums to 100%. Returns error string or null. */
function validateSplit(essentials: string, desires: string, savings: string): string | null {
  const e = parseInt(essentials, 10) || 0;
  const d = parseInt(desires, 10) || 0;
  const s = parseInt(savings, 10) || 0;
  const total = e + d + s;
  if (total !== 100) {
    return `Percentages must sum to 100%. Currently: ${total}%`;
  }
  if (e < 0 || d < 0 || s < 0) {
    return "Percentages must be non-negative";
  }
  return null;
}

export function useOnboarding(): OnboardingResult {
  const navigate = useNavigate();
  const { checkAuth } = useAuthStore();

  const [currentStep, setCurrentStep] = useState<OnboardingStep>("welcome");
  const [currency, setCurrency] = useState(DEFAULTS.currency);
  const [budgetDollars, setBudgetDollars] = useState<string>("");
  const [essentials, setEssentials] = useState<string>(String(DEFAULTS.essentials));
  const [desires, setDesires] = useState<string>(String(DEFAULTS.desires));
  const [savings, setSavings] = useState<string>(String(DEFAULTS.savings));
  const [error, setError] = useState<string | null>(null);
  const [splitError, setSplitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const stepIndex = STEP_ORDER.indexOf(currentStep);
  const totalSteps = STEP_ORDER.length;
  const isLastStep = currentStep === "split";

  function goNext() {
    if (!isLastStep) {
      setCurrentStep(STEP_ORDER[stepIndex + 1]);
    }
  }

  function goBack() {
    if (stepIndex > 0) {
      setCurrentStep(STEP_ORDER[stepIndex - 1]);
    }
  }

  function skipStep() {
    if (currentStep === "currency") {
      setCurrency(DEFAULTS.currency);
    } else if (currentStep === "budget") {
      setBudgetDollars("");
    } else if (currentStep === "split") {
      setEssentials(String(DEFAULTS.essentials));
      setDesires(String(DEFAULTS.desires));
      setSavings(String(DEFAULTS.savings));
    }
    goNext();
    if (isLastStep) {
      handleSubmit(undefined, true);
    }
  }

  async function handleSubmit(event?: FormEvent, useDefaults = false) {
    if (event) event.preventDefault();
    setError(null);

    const finalEssentials = useDefaults ? DEFAULTS.essentials : (parseInt(essentials, 10) || 0);
    const finalDesires = useDefaults ? DEFAULTS.desires : (parseInt(desires, 10) || 0);
    const finalSavings = useDefaults ? DEFAULTS.savings : (parseInt(savings, 10) || 0);
    const finalCurrency = useDefaults ? DEFAULTS.currency : currency;
    const finalBudgetDollars = useDefaults ? DEFAULTS.budgetDollars : (parseFloat(budgetDollars) || 0);

    if (!useDefaults) {
      const splitValidation = validateSplit(essentials, desires, savings);
      if (splitValidation) {
        setSplitError(splitValidation);
        return;
      }
    }

    const budgetAmountCents = Math.round(finalBudgetDollars * 100);

    setSubmitting(true);
    try {
      await apiClient("/api/finance/onboarding", {
        method: "POST",
        body: JSON.stringify({
          budgetAmount: budgetAmountCents,
          essentialsPercent: finalEssentials,
          desiresPercent: finalDesires,
          savingsPercent: finalSavings,
          currency: finalCurrency,
        }),
      });

      await apiClient("/api/auth/onboarding-complete", {
        method: "POST",
        body: JSON.stringify({
          currency: finalCurrency,
        }),
      });

      await checkAuth();
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

  const splitForm: SplitForm = {
    essentials,
    desires,
    savings,
    setEssentials,
    setDesires,
    setSavings,
    splitError,
    clearSplitError: () => setSplitError(null),
  };

  return {
    currentStep,
    stepIndex,
    totalSteps,
    currency,
    setCurrency,
    budgetDollars,
    setBudgetDollars,
    splitForm,
    goNext,
    goBack,
    skipStep,
    submit: handleSubmit,
    submitting,
    error,
  };
}
