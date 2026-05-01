import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { apiClient, ApiRequestError } from "@gofin/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
  FormDescription,
} from "@gofin/ui/components/form";

/** Supported currencies for the dropdown. */
const CURRENCIES = [
  { code: "USD", label: "US Dollar (USD)" },
  { code: "EUR", label: "Euro (EUR)" },
  { code: "GBP", label: "British Pound (GBP)" },
  { code: "JPY", label: "Japanese Yen (JPY)" },
  { code: "CAD", label: "Canadian Dollar (CAD)" },
  { code: "AUD", label: "Australian Dollar (AUD)" },
  { code: "CHF", label: "Swiss Franc (CHF)" },
  { code: "CNY", label: "Chinese Yuan (CNY)" },
  { code: "SGD", label: "Singapore Dollar (SGD)" },
  { code: "HKD", label: "Hong Kong Dollar (HKD)" },
];

/** Default values applied when a step is skipped. */
const DEFAULTS = {
  currency: "USD",
  budgetDollars: 0,
  essentials: 50,
  desires: 30,
  savings: 20,
};

type OnboardingStep = "welcome" | "currency" | "budget" | "split";

const STEP_ORDER: OnboardingStep[] = ["welcome", "currency", "budget", "split"];

export default function OnboardingPage() {
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
    // Apply defaults for the skipped step, then advance
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
    // If last step is skipped, submit with defaults
    if (isLastStep) {
      handleSubmit(undefined, true);
    }
  }

  /** Validates E/D/S split sums to 100%. Returns error string or null. */
  function validateSplit(): string | null {
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

  async function handleSubmit(event?: FormEvent, useDefaults = false) {
    if (event) event.preventDefault();
    setError(null);

    const finalEssentials = useDefaults ? DEFAULTS.essentials : (parseInt(essentials, 10) || 0);
    const finalDesires = useDefaults ? DEFAULTS.desires : (parseInt(desires, 10) || 0);
    const finalSavings = useDefaults ? DEFAULTS.savings : (parseInt(savings, 10) || 0);
    const finalCurrency = useDefaults ? DEFAULTS.currency : currency;
    const finalBudgetDollars = useDefaults ? DEFAULTS.budgetDollars : (parseFloat(budgetDollars) || 0);

    // Validate split before submitting
    if (!useDefaults) {
      const splitValidation = validateSplit();
      if (splitValidation) {
        setSplitError(splitValidation);
        return;
      }
    }

    // Convert dollars to cents
    const budgetAmountCents = Math.round(finalBudgetDollars * 100);

    setSubmitting(true);
    try {
      // Step 1: Save defaults to finance service
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

      // Step 2: Mark onboarding complete on auth service
      await apiClient("/api/auth/onboarding-complete", {
        method: "POST",
        body: JSON.stringify({
          currency: finalCurrency,
        }),
      });

      // Refresh the auth store so the user's hasCompletedOnboarding is updated
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

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-lg">
        {/* Progress indicator */}
        <div className="flex gap-1 px-6 pt-6">
          {STEP_ORDER.map((step, index) => (
            <div
              key={step}
              className={`h-1.5 flex-1 rounded-full transition-colors ${
                index <= stepIndex ? "bg-primary" : "bg-muted"
              }`}
            />
          ))}
        </div>

        {currentStep === "welcome" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Welcome to GoFin 🎉</CardTitle>
              <CardDescription>
                Let&apos;s set up your budget in a few quick steps. You can
                always change these later in Settings.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-col gap-3">
                <Button onClick={goNext} className="w-full">
                  Get started
                </Button>
                <p className="text-center text-xs text-muted-foreground">
                  Step 1 of 4
                </p>
              </div>
            </CardContent>
          </>
        )}

        {currentStep === "currency" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Default Currency</CardTitle>
              <CardDescription>
                Choose the currency you&apos;ll use most often for tracking expenses.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-col gap-4">
                <FormField>
                  <FormLabel htmlFor="currency">Currency</FormLabel>
                  <select
                    id="currency"
                    value={currency}
                    onChange={(event) => setCurrency(event.target.value)}
                    className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    {CURRENCIES.map((cur) => (
                      <option key={cur.code} value={cur.code}>
                        {cur.label}
                      </option>
                    ))}
                  </select>
                </FormField>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={goBack} className="flex-1">
                    Back
                  </Button>
                  <Button onClick={goNext} className="flex-1">
                    Continue
                  </Button>
                </div>
                <Button variant="ghost" onClick={skipStep} className="w-full">
                  Skip (use USD)
                </Button>
                <p className="text-center text-xs text-muted-foreground">
                  Step 2 of 4
                </p>
              </div>
            </CardContent>
          </>
        )}

        {currentStep === "budget" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Monthly Budget</CardTitle>
              <CardDescription>
                How much do you plan to spend each month? You can leave this at
                $0 and set it later.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-col gap-4">
                <FormField>
                  <FormLabel htmlFor="budget">Budget Amount</FormLabel>
                  <div className="relative">
                    <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                      $
                    </span>
                    <Input
                      id="budget"
                      type="number"
                      min="0"
                      step="0.01"
                      placeholder="0.00"
                      value={budgetDollars}
                      onChange={(event) => setBudgetDollars(event.target.value)}
                      className="pl-6"
                    />
                  </div>
                  <FormDescription>
                    Stored in {currency}. Enter the amount in major units (e.g., dollars).
                  </FormDescription>
                </FormField>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={goBack} className="flex-1">
                    Back
                  </Button>
                  <Button onClick={goNext} className="flex-1">
                    Continue
                  </Button>
                </div>
                <Button variant="ghost" onClick={skipStep} className="w-full">
                  Skip ($0 budget)
                </Button>
                <p className="text-center text-xs text-muted-foreground">
                  Step 3 of 4
                </p>
              </div>
            </CardContent>
          </>
        )}

        {currentStep === "split" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">E/D/S Split</CardTitle>
              <CardDescription>
                How do you want to divide your budget between Essentials,
                Desires, and Savings? The three percentages must add up to 100%.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Form onSubmit={handleSubmit}>
                <FormField>
                  <FormLabel htmlFor="essentials">Essentials %</FormLabel>
                  <Input
                    id="essentials"
                    type="number"
                    min="0"
                    max="100"
                    value={essentials}
                    onChange={(event) => {
                      setEssentials(event.target.value);
                      setSplitError(null);
                    }}
                    aria-invalid={!!splitError}
                  />
                </FormField>
                <FormField>
                  <FormLabel htmlFor="desires">Desires %</FormLabel>
                  <Input
                    id="desires"
                    type="number"
                    min="0"
                    max="100"
                    value={desires}
                    onChange={(event) => {
                      setDesires(event.target.value);
                      setSplitError(null);
                    }}
                    aria-invalid={!!splitError}
                  />
                </FormField>
                <FormField>
                  <FormLabel htmlFor="savings">Savings %</FormLabel>
                  <Input
                    id="savings"
                    type="number"
                    min="0"
                    max="100"
                    value={savings}
                    onChange={(event) => {
                      setSavings(event.target.value);
                      setSplitError(null);
                    }}
                    aria-invalid={!!splitError}
                  />
                </FormField>
                <FormDescription>
                  Total: {(parseInt(essentials, 10) || 0) + (parseInt(desires, 10) || 0) + (parseInt(savings, 10) || 0)}%
                </FormDescription>
                {splitError && <FormMessage>{splitError}</FormMessage>}
                {error && <FormMessage>{error}</FormMessage>}
                <div className="flex gap-2">
                  <Button variant="outline" onClick={goBack} type="button" className="flex-1">
                    Back
                  </Button>
                  <Button type="submit" className="flex-1" disabled={submitting}>
                    {submitting ? "Saving..." : "Complete Setup"}
                  </Button>
                </div>
                <Button variant="ghost" onClick={skipStep} type="button" className="w-full" disabled={submitting}>
                  Skip (50/30/20)
                </Button>
                <p className="text-center text-xs text-muted-foreground">
                  Step 4 of 4
                </p>
              </Form>
            </CardContent>
          </>
        )}
      </Card>
    </div>
  );
}
