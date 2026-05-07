import { useState, type FormEvent } from "react";
import { getCurrencySymbol } from "@gofin/core";
import { ApiRequestError, useBudgetSplitForm } from "@gofin/api";
import type { BudgetPeriod, UpdatePeriodRequest } from "../../../types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
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
import { Settings2 } from "lucide-react";
import { dashboardApi } from "../api";

interface BudgetSettingsEditorProps {
  period: BudgetPeriod;
  currency: string;
  onSaved: (period: BudgetPeriod) => void;
  onCancel: () => void;
}

export function BudgetSettingsEditor({
  period,
  currency,
  onSaved,
  onCancel,
}: BudgetSettingsEditorProps) {
  const currencySymbol = getCurrencySymbol(currency);

  const form = useBudgetSplitForm({
    initialBudgetCents: period.budgetAmount,
    initialSplit: {
      essentials: period.essentialsPercent,
      desires: period.desiresPercent,
      savings: period.savingsPercent,
    },
  });

  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    const validationError = form.validate();
    if (validationError) return;

    const payload = form.toPayload();
    const body: UpdatePeriodRequest = {
      budgetAmount: payload.budgetAmountCents,
      essentialsPercent: payload.essentialsPercent,
      desiresPercent: payload.desiresPercent,
      savingsPercent: payload.savingsPercent,
    };

    setSubmitting(true);
    try {
      const response = await dashboardApi.updatePeriod(period.id, body);
      onSaved(response.period);
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
    <Card data-testid="budget-settings-editor">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Settings2 className="size-4 text-muted-foreground" />
            <CardTitle className="text-base">Budget Settings</CardTitle>
          </div>
          <Button variant="ghost" size="sm" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <Form onSubmit={handleSubmit}>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
            <FormField>
              <FormLabel htmlFor="edit-budget">Monthly Budget</FormLabel>
              <div className="relative">
                <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {currencySymbol}
                </span>
                <Input
                  id="edit-budget"
                  type="number"
                  min="0"
                  step="0.01"
                  value={form.fields.budgetDollars}
                  onChange={(event) => form.setField("budgetDollars", event.target.value)}
                  className="pl-6"
                />
              </div>
            </FormField>
            <FormField>
              <FormLabel htmlFor="edit-essentials">Essentials %</FormLabel>
              <Input
                id="edit-essentials"
                type="number"
                min="0"
                max="100"
                value={form.fields.essentials}
                onChange={(event) => form.setField("essentials", event.target.value)}
              />
            </FormField>
            <FormField>
              <FormLabel htmlFor="edit-desires">Desires %</FormLabel>
              <Input
                id="edit-desires"
                type="number"
                min="0"
                max="100"
                value={form.fields.desires}
                onChange={(event) => form.setField("desires", event.target.value)}
              />
            </FormField>
            <FormField>
              <FormLabel htmlFor="edit-savings">Savings %</FormLabel>
              <Input
                id="edit-savings"
                type="number"
                min="0"
                max="100"
                value={form.fields.savings}
                onChange={(event) => form.setField("savings", event.target.value)}
              />
            </FormField>
          </div>
          <div className="mt-3 flex items-center justify-between">
            <FormDescription>Total: {form.splitTotal}%</FormDescription>
            <Button type="submit" size="sm" disabled={submitting}>
              {submitting ? "Saving..." : "Save Changes"}
            </Button>
          </div>
          {form.splitError && <FormMessage>{form.splitError}</FormMessage>}
          {error && <FormMessage>{error}</FormMessage>}
        </Form>
      </CardContent>
    </Card>
  );
}
