import { type FormEvent } from "react";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormDescription,
  FormMessage,
} from "@gofin/ui/components/form";

export interface SplitStepProps {
  essentials: string;
  desires: string;
  savings: string;
  onEssentialsChange: (value: string) => void;
  onDesiresChange: (value: string) => void;
  onSavingsChange: (value: string) => void;
  splitError: string | null;
  onClearSplitError: () => void;
  error: string | null;
  submitting: boolean;
  onSubmit: (event?: FormEvent) => void;
  onBack: () => void;
  onSkip: () => void;
}

export function SplitStep({
  essentials,
  desires,
  savings,
  onEssentialsChange,
  onDesiresChange,
  onSavingsChange,
  splitError,
  onClearSplitError,
  error,
  submitting,
  onSubmit,
  onBack,
  onSkip,
}: SplitStepProps) {
  const total =
    (parseInt(essentials, 10) || 0) +
    (parseInt(desires, 10) || 0) +
    (parseInt(savings, 10) || 0);

  return (
    <>
      <CardHeader>
        <CardTitle className="text-2xl">E/D/S Split</CardTitle>
        <CardDescription>
          How do you want to divide your budget between Essentials,
          Desires, and Savings? The three percentages must add up to 100%.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Form onSubmit={onSubmit}>
          <FormField>
            <FormLabel htmlFor="essentials">Essentials %</FormLabel>
            <Input
              id="essentials"
              type="number"
              min="0"
              max="100"
              value={essentials}
              onChange={(event) => {
                onEssentialsChange(event.target.value);
                onClearSplitError();
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
                onDesiresChange(event.target.value);
                onClearSplitError();
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
                onSavingsChange(event.target.value);
                onClearSplitError();
              }}
              aria-invalid={!!splitError}
            />
          </FormField>
          <FormDescription>
            Total: {total}%
          </FormDescription>
          {splitError && <FormMessage>{splitError}</FormMessage>}
          {error && <FormMessage>{error}</FormMessage>}
          <div className="flex gap-2">
            <Button variant="outline" onClick={onBack} type="button" className="flex-1">
              Back
            </Button>
            <Button type="submit" className="flex-1" disabled={submitting}>
              {submitting ? "Saving..." : "Complete Setup"}
            </Button>
          </div>
          <Button variant="ghost" onClick={onSkip} type="button" className="w-full" disabled={submitting}>
            Skip (50/30/20)
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            Step 4 of 4
          </p>
        </Form>
      </CardContent>
    </>
  );
}
