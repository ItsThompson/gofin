import { useState, type FormEvent } from "react";
import {
  getCurrencyInputStep,
  getCurrencySymbol,
  getMinorUnitDigits,
} from "@gofin/core";
import { useBudgetSplitForm, useSupportedCurrencyOptions } from "@gofin/api";
import type { User } from "@gofin/core";
import type { DefaultSettings, CreatePeriodRequest } from "@gofin/core";
import { initialReportingCurrency } from "../utils/initialReportingCurrency";
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
  const [reportingCurrency, setReportingCurrency] = useState(() =>
    initialReportingCurrency(defaults, user),
  );
  const [currencyError, setCurrencyError] = useState<string | null>(null);
  const currencySymbol = getCurrencySymbol(reportingCurrency);
  const currencyOptions = useSupportedCurrencyOptions();

  const form = useBudgetSplitForm({
    initialBudgetCents: defaults?.budgetAmount || undefined,
    currency: reportingCurrency,
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

    if (!reportingCurrency) {
      setCurrencyError("Reporting currency is required");
      return;
    }

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
      reportingCurrency,
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
                  step={getCurrencyInputStep(reportingCurrency)}
                  placeholder={getMinorUnitDigits(reportingCurrency) === 0 ? "0" : "0.00"}
                  value={form.fields.budgetDollars}
                  onChange={(event) => form.setField("budgetDollars", event.target.value)}
                  className="pl-6"
                />
              </div>
            </FormField>

            <FormField>
              <FormLabel htmlFor="reporting-currency">Reporting Currency</FormLabel>
              <select
                id="reporting-currency"
                value={reportingCurrency}
                onChange={(event) => {
                  setReportingCurrency(event.target.value);
                  setCurrencyError(null);
                }}
                aria-invalid={!!currencyError}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {!reportingCurrency && <option value="">Select a currency</option>}
                {currencyOptions.map((option) => (
                  <option key={option.code} value={option.code}>
                    {option.label}
                  </option>
                ))}
              </select>
              {currencyError && <FormMessage>{currencyError}</FormMessage>}
              <FormDescription>
                Reporting currency cannot be changed after this period is created.
                Default budget changes only apply to future periods.
              </FormDescription>
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
