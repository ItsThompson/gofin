import { getCurrencySymbol } from "@gofin/core";
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
} from "@gofin/ui/components/form";
import { PlusCircle } from "lucide-react";
import type { FinancePageProps } from "../../types/pages";
import { useNewExpenseForm, EXPENSE_TYPES } from "./hooks/useNewExpenseForm";

/**
 * New expense form page. Allows users to log a standard expense.
 * Amount is entered in dollars (with decimals) and converted to cents
 * before submission. On success, redirects to /dashboard.
 */
export function NewExpenseFeature({ user }: FinancePageProps) {
  const currencySymbol = getCurrencySymbol(user.currency);
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;
  const { state, actions } = useNewExpenseForm(user.currency);

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
            {new Date(currentYear, currentMonth - 1).toLocaleString("en-US", {
              month: "long",
            })}{" "}
            {currentYear}.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form onSubmit={actions.handleSubmit}>
            {/* Name */}
            <FormField>
              <FormLabel htmlFor="expense-name">Name</FormLabel>
              <Input
                id="expense-name"
                type="text"
                placeholder="e.g. Grocery shopping"
                value={state.name}
                onChange={(event) => {
                  actions.setName(event.target.value);
                  actions.clearFieldError("name");
                }}
                aria-invalid={!!state.fieldErrors.name}
              />
              <FormMessage>{state.fieldErrors.name}</FormMessage>
            </FormField>

            {/* Amount */}
            <FormField>
              <FormLabel htmlFor="expense-amount">Amount</FormLabel>
              <div className="relative">
                <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {currencySymbol}
                </span>
                <Input
                  id="expense-amount"
                  type="number"
                  min="0.01"
                  step="0.01"
                  placeholder="0.00"
                  value={state.amountDollars}
                  onChange={(event) => {
                    actions.setAmountDollars(event.target.value);
                    actions.clearFieldError("amount");
                  }}
                  className="pl-6"
                  aria-invalid={!!state.fieldErrors.amount}
                />
              </div>
              <FormMessage>{state.fieldErrors.amount}</FormMessage>
            </FormField>

            {/* Expense Type (Radio) */}
            <FormField>
              <FormLabel>Type</FormLabel>
              <div className="flex gap-4" role="radiogroup" aria-label="Expense type">
                {EXPENSE_TYPES.map((type) => (
                  <label
                    key={type}
                    className="flex cursor-pointer items-center gap-2"
                  >
                    <input
                      type="radio"
                      name="expenseType"
                      value={type}
                      checked={state.expenseType === type}
                      onChange={() => actions.setExpenseType(type)}
                      className="size-4 accent-primary"
                    />
                    <span className="text-sm capitalize">{type}</span>
                  </label>
                ))}
              </div>
            </FormField>

            {/* Tag (Dropdown) */}
            <FormField>
              <FormLabel htmlFor="expense-tag">Tag</FormLabel>
              <select
                id="expense-tag"
                value={state.tagId}
                onChange={(event) => {
                  actions.setTagId(event.target.value);
                  actions.clearFieldError("tagId");
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

            {/* Date */}
            <FormField>
              <FormLabel htmlFor="expense-date">Date</FormLabel>
              <Input
                id="expense-date"
                type="date"
                value={state.expenseDate}
                onChange={(event) => {
                  actions.setExpenseDate(event.target.value);
                  actions.clearFieldError("expenseDate");
                }}
                aria-invalid={!!state.fieldErrors.expenseDate}
              />
              <FormMessage>{state.fieldErrors.expenseDate}</FormMessage>
            </FormField>

            {/* Pro-rata Toggle */}
            <FormField>
              <label className="flex cursor-pointer items-center gap-2">
                <input
                  type="checkbox"
                  checked={state.isProRata}
                  onChange={(event) => actions.setIsProRata(event.target.checked)}
                  className="size-4 accent-primary"
                />
                <span className="text-sm font-medium">Spread across months</span>
              </label>
            </FormField>

            {/* Pro-rata Months */}
            {state.isProRata && (
              <FormField>
                <FormLabel htmlFor="pro-rata-months">Number of months</FormLabel>
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

            {/* Error Message */}
            {state.error && <FormMessage>{state.error}</FormMessage>}

            {/* Submit */}
            <Button type="submit" className="w-full" disabled={state.submitting}>
              {state.submitting ? "Saving..." : "Log Expense"}
            </Button>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
