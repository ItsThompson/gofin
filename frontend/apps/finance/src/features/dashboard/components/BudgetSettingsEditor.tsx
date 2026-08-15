import { type FormEvent } from "react";
import { getCurrencyInputStep, getCurrencySymbol } from "@gofin/core";
import { useBudgetSplitForm, useFormMutation } from "@gofin/api";
import type { BudgetPeriod, PeriodResponse, UpdatePeriodRequest } from "@gofin/core";
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
  onSaved: (period: BudgetPeriod) => void;
  onCancel: () => void;
}

export function BudgetSettingsEditor({
  period,
  onSaved,
  onCancel,
}: BudgetSettingsEditorProps) {
  const currencySymbol = getCurrencySymbol(period.reportingCurrency);

  const form = useBudgetSplitForm({
    initialBudgetCents: period.budgetAmount,
    currency: period.reportingCurrency,
    initialSplit: {
      essentials: period.essentialsPercent,
      desires: period.desiresPercent,
      savings: period.savingsPercent,
    },
  });

  const mutation = useFormMutation<PeriodResponse>({
    onSuccess: (response) => onSaved(response.period),
  });

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    mutation.clearError();

    const validationError = form.validate();
    if (validationError) return;

    const payload = form.toPayload();
    const body: UpdatePeriodRequest = {
      budgetAmount: payload.budgetAmountCents,
      essentialsPercent: payload.essentialsPercent,
      desiresPercent: payload.desiresPercent,
      savingsPercent: payload.savingsPercent,
    };

    mutation.submit(() => dashboardApi.updatePeriod(period.id, body));
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
                  step={getCurrencyInputStep(period.reportingCurrency)}
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
            <Button type="submit" size="sm" disabled={mutation.submitting}>
              {mutation.submitting ? "Saving..." : "Save Changes"}
            </Button>
          </div>
          {form.splitError && <FormMessage>{form.splitError}</FormMessage>}
          {mutation.error && <FormMessage>{mutation.error}</FormMessage>}
        </Form>
      </CardContent>
    </Card>
  );
}
