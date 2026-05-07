import { type FormEvent } from "react";
import { getCurrencySymbol } from "@gofin/core";
import { useBudgetSplitForm } from "@gofin/api";
import type { User } from "@gofin/core";
import type { DefaultSettings, CreatePeriodRequest } from "@/types";
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

interface CreatePeriodPromptProps {
  defaults: DefaultSettings | null;
  user: User;
  year: number;
  month: number;
  onCreatePeriod: (body: CreatePeriodRequest) => void;
  creating: boolean;
  createError: string | null;
}

export function CreatePeriodPrompt({
  defaults,
  user,
  year,
  month,
  onCreatePeriod,
  creating,
  createError,
}: CreatePeriodPromptProps) {
  const isZeroBudget = !defaults || defaults.budgetAmount === 0;
  const currencySymbol = getCurrencySymbol(user.currency);

  const form = useBudgetSplitForm({
    initialBudgetCents: defaults?.budgetAmount || undefined,
    initialSplit: defaults
      ? {
          essentials: defaults.essentialsPercent,
          desires: defaults.desiresPercent,
          savings: defaults.savingsPercent,
        }
      : undefined,
  });

  const monthName = new Date(year, month - 1).toLocaleString("en-US", {
    month: "long",
  });

  function handleSubmit(event: FormEvent) {
    event.preventDefault();

    const validationError = form.validate();
    if (validationError) return;

    const payload = form.toPayload();
    const body: CreatePeriodRequest = {
      year,
      month,
      budgetAmount: payload.budgetAmountCents,
      essentialsPercent: payload.essentialsPercent,
      desiresPercent: payload.desiresPercent,
      savingsPercent: payload.savingsPercent,
    };

    onCreatePeriod(body);
  }

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
                  value={form.fields.budgetDollars}
                  onChange={(event) => form.setField("budgetDollars", event.target.value)}
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
                value={form.fields.essentials}
                onChange={(event) => form.setField("essentials", event.target.value)}
                aria-invalid={!!form.splitError}
              />
            </FormField>

            <FormField>
              <FormLabel htmlFor="desires">Desires %</FormLabel>
              <Input
                id="desires"
                type="number"
                min="0"
                max="100"
                value={form.fields.desires}
                onChange={(event) => form.setField("desires", event.target.value)}
                aria-invalid={!!form.splitError}
              />
            </FormField>

            <FormField>
              <FormLabel htmlFor="savings">Savings %</FormLabel>
              <Input
                id="savings"
                type="number"
                min="0"
                max="100"
                value={form.fields.savings}
                onChange={(event) => form.setField("savings", event.target.value)}
                aria-invalid={!!form.splitError}
              />
            </FormField>

            <FormDescription>Total: {form.splitTotal}%</FormDescription>
            {form.splitError && <FormMessage>{form.splitError}</FormMessage>}
            {createError && <FormMessage>{createError}</FormMessage>}

            <Button type="submit" className="w-full" disabled={creating}>
              {creating ? "Creating..." : `Create ${monthName} Period`}
            </Button>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
