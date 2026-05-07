import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  FormField,
  FormLabel,
  FormDescription,
} from "@gofin/ui/components/form";

export interface BudgetStepProps {
  budgetDollars: string;
  onBudgetChange: (value: string) => void;
  currency: string;
  onNext: () => void;
  onBack: () => void;
  onSkip: () => void;
}

export function BudgetStep({
  budgetDollars,
  onBudgetChange,
  currency,
  onNext,
  onBack,
  onSkip,
}: BudgetStepProps) {
  return (
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
                onChange={(event) => onBudgetChange(event.target.value)}
                className="pl-6"
              />
            </div>
            <FormDescription>
              Stored in {currency}. Enter the amount in major units (e.g., dollars).
            </FormDescription>
          </FormField>
          <div className="flex gap-2">
            <Button variant="outline" onClick={onBack} className="flex-1">
              Back
            </Button>
            <Button onClick={onNext} className="flex-1">
              Continue
            </Button>
          </div>
          <Button variant="ghost" onClick={onSkip} className="w-full">
            Skip ($0 budget)
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            Step 3 of 4
          </p>
        </div>
      </CardContent>
    </>
  );
}
