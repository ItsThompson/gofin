import { SUPPORTED_CURRENCIES } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import {
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import { FormField, FormLabel } from "@gofin/ui/components/form";

/** Currency options derived from SUPPORTED_CURRENCIES with display name format. */
const CURRENCIES = SUPPORTED_CURRENCIES.map((currency) => ({
  code: currency.code,
  label: `${currency.name} (${currency.code})`,
}));

export interface CurrencyStepProps {
  currency: string;
  onCurrencyChange: (code: string) => void;
  onNext: () => void;
  onBack: () => void;
  onSkip: () => void;
}

export function CurrencyStep({
  currency,
  onCurrencyChange,
  onNext,
  onBack,
  onSkip,
}: CurrencyStepProps) {
  return (
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
              onChange={(event) => onCurrencyChange(event.target.value)}
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
            <Button variant="outline" onClick={onBack} className="flex-1">
              Back
            </Button>
            <Button onClick={onNext} className="flex-1">
              Continue
            </Button>
          </div>
          <Button variant="ghost" onClick={onSkip} className="w-full">
            Skip (use USD)
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            Step 2 of 4
          </p>
        </div>
      </CardContent>
    </>
  );
}
