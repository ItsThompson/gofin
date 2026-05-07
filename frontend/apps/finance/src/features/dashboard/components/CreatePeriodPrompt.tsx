import { useState, type FormEvent } from "react";
import { getCurrencySymbol } from "@gofin/core";
import { ApiRequestError } from "@gofin/api";
import type { User } from "@gofin/core";
import type { DefaultSettings, BudgetPeriod, CreatePeriodRequest } from "@/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
  FormDescription,
} from "@gofin/ui/components/form";
import { AlertTriangle } from "lucide-react";
import { dashboardApi } from "../api";

interface CreatePeriodPromptProps {
  defaults: DefaultSettings | null;
  user: User;
  year: number;
  month: number;
  onPeriodCreated: (period: BudgetPeriod) => void;
}

const FALLBACK_DEFAULTS = {
  budgetAmount: 0,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
};

export function CreatePeriodPrompt({
  defaults,
  user,
  year,
  month,
  onPeriodCreated,
}: CreatePeriodPromptProps) {
  const effectiveDefaults = defaults ?? FALLBACK_DEFAULTS;
  const isZeroBudget = effectiveDefaults.budgetAmount === 0;
  const currencySymbol = getCurrencySymbol(user.currency);

  const [budgetDollars, setBudgetDollars] = useState<string>(
    effectiveDefaults.budgetAmount > 0
      ? (effectiveDefaults.budgetAmount / 100).toString()
      : "",
  );
  const [essentials, setEssentials] = useState<string>(
    String(effectiveDefaults.essentialsPercent),
  );
  const [desires, setDesires] = useState<string>(
    String(effectiveDefaults.desiresPercent),
  );
  const [savings, setSavings] = useState<string>(
    String(effectiveDefaults.savingsPercent),
  );
  const [splitError, setSplitError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const monthName = new Date(year, month - 1).toLocaleString("en-US", {
    month: "long",
  });

  function validateSplit(): string | null {
    const essentialsVal = parseInt(essentials, 10) || 0;
    const desiresVal = parseInt(desires, 10) || 0;
    const savingsVal = parseInt(savings, 10) || 0;
    const total = essentialsVal + desiresVal + savingsVal;
    if (total !== 100) {
      return `Percentages must sum to 100%. Currently: ${total}%`;
    }
    if (essentialsVal < 0 || desiresVal < 0 || savingsVal < 0) {
      return "Percentages must be non-negative";
    }
    return null;
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    const splitValidation = validateSplit();
    if (splitValidation) {
      setSplitError(splitValidation);
      return;
    }

    const budgetAmountCents = Math.round(
      (parseFloat(budgetDollars) || 0) * 100,
    );

    const body: CreatePeriodRequest = {
      year,
      month,
      budgetAmount: budgetAmountCents,
      essentialsPercent: parseInt(essentials, 10) || 0,
      desiresPercent: parseInt(desires, 10) || 0,
      savingsPercent: parseInt(savings, 10) || 0,
    };

    setSubmitting(true);
    try {
      const response = await dashboardApi.createPeriod(body);

      if (response.autoCreatedPeriods && response.autoCreatedPeriods > 0 && response.autoCreatedMonths) {
        const monthsList = response.autoCreatedMonths.join(", ");
        alert(`${response.autoCreatedPeriods} period(s) were automatically created for ${monthsList} with your default settings.`);
      }

      onPeriodCreated(response.period);
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

  const splitTotal =
    (parseInt(essentials, 10) || 0) +
    (parseInt(desires, 10) || 0) +
    (parseInt(savings, 10) || 0);

  return (
    <div className="flex items-start justify-center pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle className="text-2xl">
            Set Up {monthName} {year}
          </CardTitle>
          <CardDescription>
            No budget period exists for this month. Confirm your settings or
            customize them below.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isZeroBudget && (
            <div className="mb-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <span>
                No budget configured yet. Set an amount below, or create the
                period at $0 and update it later in Settings.
              </span>
            </div>
          )}

          <Form onSubmit={handleSubmit}>
            <FormField>
              <FormLabel htmlFor="budget">Monthly Budget</FormLabel>
              <div className="relative">
                <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {currencySymbol}
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
            </FormField>

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

            <FormDescription>Total: {splitTotal}%</FormDescription>
            {splitError && <FormMessage>{splitError}</FormMessage>}
            {error && <FormMessage>{error}</FormMessage>}

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "Creating..." : `Create ${monthName} Period`}
            </Button>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
