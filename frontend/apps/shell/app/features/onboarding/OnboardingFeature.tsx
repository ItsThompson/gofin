import { Card } from "@gofin/ui/components/card";
import { useOnboarding } from "./hooks/useOnboarding";
import type { OnboardingStep } from "./hooks/useOnboarding";
import { WelcomeStep } from "./steps/WelcomeStep";
import { CurrencyStep } from "./steps/CurrencyStep";
import { BudgetStep } from "./steps/BudgetStep";
import { SplitStep } from "./steps/SplitStep";

const STEP_ORDER: OnboardingStep[] = ["welcome", "currency", "budget", "split"];

export function OnboardingFeature() {
  const {
    currentStep,
    stepIndex,
    currency,
    setCurrency,
    budgetDollars,
    setBudgetDollars,
    splitForm,
    goNext,
    goBack,
    skipStep,
    submit,
    submitting,
    error,
  } = useOnboarding();

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
          <WelcomeStep onNext={goNext} />
        )}

        {currentStep === "currency" && (
          <CurrencyStep
            currency={currency}
            onCurrencyChange={setCurrency}
            onNext={goNext}
            onBack={goBack}
            onSkip={skipStep}
          />
        )}

        {currentStep === "budget" && (
          <BudgetStep
            budgetDollars={budgetDollars}
            onBudgetChange={setBudgetDollars}
            currency={currency}
            onNext={goNext}
            onBack={goBack}
            onSkip={skipStep}
          />
        )}

        {currentStep === "split" && (
          <SplitStep
            essentials={splitForm.essentials}
            desires={splitForm.desires}
            savings={splitForm.savings}
            onEssentialsChange={splitForm.setEssentials}
            onDesiresChange={splitForm.setDesires}
            onSavingsChange={splitForm.setSavings}
            splitError={splitForm.splitError}
            onClearSplitError={splitForm.clearSplitError}
            error={error}
            submitting={submitting}
            onSubmit={submit}
            onBack={goBack}
            onSkip={skipStep}
          />
        )}
      </Card>
    </div>
  );
}
