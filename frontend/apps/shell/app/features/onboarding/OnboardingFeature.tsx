import { Card } from "@gofin/ui/components/card";
import { useOnboarding } from "./hooks/useOnboarding";
import type { OnboardingStep } from "./hooks/useOnboarding";
import { WelcomeStep } from "./steps/WelcomeStep";
import { CurrencyStep } from "./steps/CurrencyStep";
import { BudgetStep } from "./steps/BudgetStep";
import { SplitStep } from "./steps/SplitStep";

const STEP_ORDER: OnboardingStep[] = ["welcome", "currency", "budget", "split"];

export function OnboardingFeature() {
  const { state, actions } = useOnboarding();

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-lg">
        {/* Progress indicator */}
        <div className="flex gap-1 px-6 pt-6">
          {STEP_ORDER.map((step, index) => (
            <div
              key={step}
              className={`h-1.5 flex-1 rounded-full transition-colors ${
                index <= state.stepIndex ? "bg-primary" : "bg-muted"
              }`}
            />
          ))}
        </div>

        {state.currentStep === "welcome" && (
          <WelcomeStep onNext={actions.goNext} />
        )}

        {state.currentStep === "currency" && (
          <CurrencyStep
            currency={state.currency}
            onCurrencyChange={actions.setCurrency}
            onNext={actions.goNext}
            onBack={actions.goBack}
            onSkip={actions.skipStep}
          />
        )}

        {state.currentStep === "budget" && (
          <BudgetStep
            budgetDollars={state.budgetDollars}
            onBudgetChange={actions.setBudgetDollars}
            currency={state.currency}
            onNext={actions.goNext}
            onBack={actions.goBack}
            onSkip={actions.skipStep}
          />
        )}

        {state.currentStep === "split" && (
          <SplitStep
            essentials={state.splitForm.essentials}
            desires={state.splitForm.desires}
            savings={state.splitForm.savings}
            onEssentialsChange={state.splitForm.setEssentials}
            onDesiresChange={state.splitForm.setDesires}
            onSavingsChange={state.splitForm.setSavings}
            splitError={state.splitForm.splitError}
            onClearSplitError={state.splitForm.clearSplitError}
            error={state.error}
            submitting={state.submitting}
            onSubmit={actions.submit}
            onBack={actions.goBack}
            onSkip={actions.skipStep}
          />
        )}
      </Card>
    </div>
  );
}
