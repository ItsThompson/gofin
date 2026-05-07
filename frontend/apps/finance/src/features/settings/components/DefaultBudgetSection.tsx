import type { User } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import { Check, Loader2 } from "lucide-react";
import { useDefaultBudget } from "../hooks/useDefaultBudget";

const CURRENCY_OPTIONS = [
  { code: "USD", label: "USD ($)" },
  { code: "EUR", label: "EUR (€)" },
  { code: "GBP", label: "GBP (£)" },
  { code: "JPY", label: "JPY (¥)" },
  { code: "CAD", label: "CAD (C$)" },
  { code: "AUD", label: "AUD (A$)" },
  { code: "CHF", label: "CHF" },
  { code: "CNY", label: "CNY (¥)" },
  { code: "SGD", label: "SGD (S$)" },
  { code: "HKD", label: "HKD (HK$)" },
];

export function DefaultBudgetSection({ user }: { user: User }) {
  const { state, actions } = useDefaultBudget(user);

  if (state.fetching) {
    return (
      <div className="flex items-center gap-2 py-8 text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        <span>Loading defaults...</span>
      </div>
    );
  }

  return (
    <Form onSubmit={actions.handleSubmit}>
      <FormField>
        <FormLabel htmlFor="budget-amount">Monthly Budget</FormLabel>
        <Input
          id="budget-amount"
          type="number"
          min="0"
          step="0.01"
          value={state.budgetDollars}
          onChange={(event) => actions.setBudgetDollars(event.target.value)}
        />
      </FormField>

      <div className="grid grid-cols-3 gap-3">
        <FormField>
          <FormLabel htmlFor="essentials-pct">Essentials %</FormLabel>
          <Input
            id="essentials-pct"
            type="number"
            min="0"
            max="100"
            value={state.essentials}
            onChange={(event) => actions.setEssentials(event.target.value)}
          />
        </FormField>
        <FormField>
          <FormLabel htmlFor="desires-pct">Desires %</FormLabel>
          <Input
            id="desires-pct"
            type="number"
            min="0"
            max="100"
            value={state.desires}
            onChange={(event) => actions.setDesires(event.target.value)}
          />
        </FormField>
        <FormField>
          <FormLabel htmlFor="savings-pct">Savings %</FormLabel>
          <Input
            id="savings-pct"
            type="number"
            min="0"
            max="100"
            value={state.savings}
            onChange={(event) => actions.setSavings(event.target.value)}
          />
        </FormField>
      </div>

      <FormField>
        <FormLabel htmlFor="currency-select">Currency</FormLabel>
        <select
          id="currency-select"
          value={state.currency}
          onChange={(event) => actions.setCurrency(event.target.value)}
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {CURRENCY_OPTIONS.map((opt) => (
            <option key={opt.code} value={opt.code}>
              {opt.label}
            </option>
          ))}
        </select>
      </FormField>

      <FormMessage>{state.error}</FormMessage>

      {state.success && (
        <p className="flex items-center gap-1.5 text-sm text-green-600">
          <Check className="size-4" />
          Default settings updated successfully.
        </p>
      )}

      <Button type="submit" disabled={state.loading}>
        {state.loading && <Loader2 className="size-4 animate-spin" />}
        Save Defaults
      </Button>
    </Form>
  );
}
