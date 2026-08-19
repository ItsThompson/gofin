import {
  getCurrencyInputStep,
  getMinorUnitDigits,
  SUPPORTED_CURRENCY_OPTIONS,
} from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import { PlusCircle } from "lucide-react";
import { ExpenseNameCombobox } from "../../expense-autocomplete";
import {
  EXPENSE_TYPES,
  type NewExpenseFormActions,
  type NewExpenseFormState,
} from "../hooks/useNewExpenseForm";

interface ExpenseFormProps {
  state: NewExpenseFormState;
  actions: NewExpenseFormActions;
  currencySymbol: string;
  year: number;
  month: number;
}

export function ExpenseForm({
  state,
  actions,
  currencySymbol,
  year,
  month,
}: ExpenseFormProps) {
  return (
    <div className="flex items-start justify-center pt-4 md:pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <div className="flex items-center gap-3">
            <PlusCircle className="size-6 text-primary" />
            <CardTitle className="text-2xl">New Expense</CardTitle>
          </div>
          <CardDescription>
            Log an expense for{" "}
            {new Date(year, month - 1).toLocaleString("en-US", {
              month: "long",
            })}{" "}
            {year}.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form onSubmit={actions.handleSubmit} noValidate>
            <FormField>
              <FormLabel htmlFor="expense-name">Name</FormLabel>
              <ExpenseNameCombobox
                value={state.fields.name}
                onValueChange={(value) => {
                  actions.setField("name", value);
                  actions.clearFieldError("name");
                }}
                onSelectSuggestion={actions.applySuggestion}
                error={state.fieldErrors.name}
              />
            </FormField>

            <FormField>
              <FormLabel htmlFor="expense-amount">Amount</FormLabel>
              <div className="relative">
                <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {currencySymbol}
                </span>
                <Input
                  id="expense-amount"
                  type="number"
                  min={getMinorUnitDigits(state.transactionCurrency) === 0 ? "1" : "0.01"}
                  step={getCurrencyInputStep(state.transactionCurrency)}
                  placeholder={getMinorUnitDigits(state.transactionCurrency) === 0 ? "0" : "0.00"}
                  value={state.fields.amountDollars}
                  onChange={(event) => {
                    actions.setField("amountDollars", event.target.value);
                  }}
                  className="pl-6"
                  aria-invalid={!!state.fieldErrors.amount}
                />
              </div>
              <FormMessage>{state.fieldErrors.amount}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="transaction-currency">Transaction Currency</FormLabel>
              <select
                id="transaction-currency"
                value={state.transactionCurrency}
                onChange={(event) => actions.setTransactionCurrency(event.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {SUPPORTED_CURRENCY_OPTIONS.map((option) => (
                  <option key={option.code} value={option.code}>
                    {option.label}
                  </option>
                ))}
              </select>
            </FormField>

            <FormField>
              <FormLabel>Type</FormLabel>
              <div
                className="flex gap-4"
                role="radiogroup"
                aria-label="Expense type"
              >
                {EXPENSE_TYPES.map((type) => (
                  <label
                    key={type}
                    className="flex cursor-pointer items-center gap-2"
                  >
                    <input
                      type="radio"
                      name="expenseType"
                      value={type}
                      checked={state.fields.expenseType === type}
                      onChange={() => actions.setField("expenseType", type)}
                      className="size-4 accent-primary"
                    />
                    <span className="text-sm capitalize">{type}</span>
                  </label>
                ))}
              </div>
            </FormField>

            <FormField>
              <FormLabel htmlFor="expense-tag">Tag</FormLabel>
              <select
                id="expense-tag"
                value={state.fields.tagId}
                onChange={(event) => {
                  actions.setField("tagId", event.target.value);
                }}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                aria-invalid={!!state.fieldErrors.tagId}
              >
                {state.tagsLoading ? (
                  <option value="">Loading tags...</option>
                ) : (
                  state.tags.map((tag) => (
                    <option key={tag.id} value={tag.id}>
                      {tag.name}
                    </option>
                  ))
                )}
              </select>
              <FormMessage>{state.fieldErrors.tagId}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="expense-date">Date</FormLabel>
              <Input
                id="expense-date"
                type="date"
                value={state.fields.expenseDate}
                onChange={(event) => {
                  actions.setField("expenseDate", event.target.value);
                }}
                aria-invalid={!!state.fieldErrors.expenseDate}
              />
              <FormMessage>{state.fieldErrors.expenseDate}</FormMessage>
            </FormField>

            <FormField>
              <label className="flex cursor-pointer items-center gap-2">
                <input
                  type="checkbox"
                  checked={state.isProRata}
                  onChange={(event) =>
                    actions.setIsProRata(event.target.checked)
                  }
                  className="size-4 accent-primary"
                />
                <span className="text-sm font-medium">
                  Spread across months
                </span>
              </label>
            </FormField>

            {state.isProRata && (
              <FormField>
                <FormLabel htmlFor="pro-rata-months">
                  Number of months
                </FormLabel>
                <Input
                  id="pro-rata-months"
                  type="number"
                  min="2"
                  step="1"
                  placeholder="e.g. 3"
                  value={state.proRataMonths}
                  onChange={(event) => {
                    actions.setProRataMonths(event.target.value);
                    actions.clearFieldError("proRataMonths");
                  }}
                  aria-invalid={!!state.fieldErrors.proRataMonths}
                />
                <FormMessage>{state.fieldErrors.proRataMonths}</FormMessage>
              </FormField>
            )}

            {state.error && <FormMessage>{state.error}</FormMessage>}

            <Button
              type="submit"
              className="w-full"
              disabled={state.submitting}
            >
              {state.submitting ? "Saving..." : "Log Expense"}
            </Button>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
