import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { apiClient, useBudgetSplitForm, useFormMutation } from "@gofin/api";
import { DEFAULT_BUDGET_SPLIT } from "@gofin/core";
import type {
  OnboardingStep,
  OnboardingState,
  OnboardingActions,
  SplitForm,
} from "./types";

export type { OnboardingStep, OnboardingState, OnboardingActions, SplitForm } from "./types";

const STEP_ORDER: OnboardingStep[] = ["welcome", "currency", "budget", "split"];

/** Default values applied when a step is skipped. */
const DEFAULTS = {
  currency: "USD",
  budgetDollars: 0,
};

export function useOnboarding(): { state: OnboardingState; actions: OnboardingActions } {
  const navigate = useNavigate();
  const { checkAuth } = useAuthStore();

  const [currentStep, setCurrentStep] = useState<OnboardingStep>("welcome");
  const [currency, setCurrency] = useState(DEFAULTS.currency);
  const [budgetDollars, setBudgetDollars] = useState<string>("");
  const form = useBudgetSplitForm();

  const mutation = useFormMutation<void>({
    onSuccess: async () => {
      await checkAuth();
      navigate("/dashboard");
    },
  });

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
      form.reset();
    }
    goNext();
    if (isLastStep) {
      handleSubmit(undefined, true);
    }
  }

  function handleSubmit(event?: FormEvent, useDefaults = false) {
    if (event) event.preventDefault();

    const finalEssentials = useDefaults ? DEFAULT_BUDGET_SPLIT.essentials : (parseInt(form.fields.essentials, 10) || 0);
    const finalDesires = useDefaults ? DEFAULT_BUDGET_SPLIT.desires : (parseInt(form.fields.desires, 10) || 0);
    const finalSavings = useDefaults ? DEFAULT_BUDGET_SPLIT.savings : (parseInt(form.fields.savings, 10) || 0);
    const finalCurrency = useDefaults ? DEFAULTS.currency : currency;
    const finalBudgetDollars = useDefaults ? DEFAULTS.budgetDollars : (parseFloat(budgetDollars) || 0);

    if (!useDefaults) {
      const validationError = form.validate();
      if (validationError) return;
    }

    const budgetAmountCents = Math.round(finalBudgetDollars * 100);

    mutation.submit(async () => {
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
    });
  }

  const splitForm: SplitForm = {
    essentials: form.fields.essentials,
    desires: form.fields.desires,
    savings: form.fields.savings,
    setEssentials: (value: string) => form.setField("essentials", value),
    setDesires: (value: string) => form.setField("desires", value),
    setSavings: (value: string) => form.setField("savings", value),
    splitError: form.splitError,
    clearSplitError: () => {},  // No-op: splitError is derived and auto-clears when fields become valid
  };

  return {
    state: {
      currentStep,
      stepIndex,
      totalSteps,
      currency,
      budgetDollars,
      splitForm,
      submitting: mutation.submitting,
      error: mutation.error,
    },
    actions: {
      setCurrency,
      setBudgetDollars,
      goNext,
      goBack,
      skipStep,
      submit: handleSubmit,
    },
  };
}
