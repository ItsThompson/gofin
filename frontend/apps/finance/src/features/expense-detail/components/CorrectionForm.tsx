import type { FormEvent } from "react";
import { getCurrencySymbol, EXPENSE_TYPES } from "@gofin/core";
import type { ExpenseFields } from "../../../lib/validate-expense-fields";
import type { Tag } from "../../../types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import {
  ExpenseNameCombobox,
  type ExpenseSuggestion,
} from "../../expense-autocomplete";

interface CorrectionFormProps {
  currency: string;
  tags: Tag[];
  fields: ExpenseFields;
  fieldErrors: Record<string, string>;
  submitting: boolean;
  submitError: string | null;
  onCancel: () => void;
  onSubmit: (event: FormEvent) => void;
  onFieldChange: (key: keyof ExpenseFields, value: string) => void;
  onSelectSuggestion: (suggestion: ExpenseSuggestion) => void;
}

export function CorrectionForm({
  currency,
  tags,
  fields,
  fieldErrors,
  submitting,
  submitError,
  onCancel,
  onSubmit,
  onFieldChange,
  onSelectSuggestion,
}: CorrectionFormProps) {
  const currencySymbol = getCurrencySymbol(currency);

  return (
    <Form onSubmit={onSubmit}>
      {/* Name */}
      <FormField>
        <FormLabel htmlFor="correct-name">Name</FormLabel>
        <ExpenseNameCombobox
          id="correct-name"
          value={fields.name}
          onValueChange={(value) => onFieldChange("name", value)}
          onSelectSuggestion={onSelectSuggestion}
          error={fieldErrors.name}
        />
      </FormField>

      {/* Amount */}
      <FormField>
        <FormLabel htmlFor="correct-amount">Amount</FormLabel>
        <div className="relative">
          <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
            {currencySymbol}
          </span>
          <Input
            id="correct-amount"
            type="number"
            min="0.01"
            step="0.01"
            value={fields.amountDollars}
            onChange={(event) =>
              onFieldChange("amountDollars", event.target.value)
            }
            className="pl-6"
            aria-invalid={!!fieldErrors.amount}
          />
        </div>
        <FormMessage>{fieldErrors.amount}</FormMessage>
      </FormField>

      {/* Expense Type */}
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
                name="correctExpenseType"
                value={type}
                checked={fields.expenseType === type}
                onChange={() => onFieldChange("expenseType", type)}
                className="size-4 accent-primary"
              />
              <span className="text-sm capitalize">{type}</span>
            </label>
          ))}
        </div>
      </FormField>

      {/* Tag */}
      <FormField>
        <FormLabel htmlFor="correct-tag">Tag</FormLabel>
        <select
          id="correct-tag"
          value={fields.tagId}
          onChange={(event) => onFieldChange("tagId", event.target.value)}
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          aria-invalid={!!fieldErrors.tagId}
        >
          {tags.map((tag) => (
            <option key={tag.id} value={tag.id}>
              {tag.name}
            </option>
          ))}
        </select>
        <FormMessage>{fieldErrors.tagId}</FormMessage>
      </FormField>

      {/* Date */}
      <FormField>
        <FormLabel htmlFor="correct-date">Date</FormLabel>
        <Input
          id="correct-date"
          type="date"
          value={fields.expenseDate}
          onChange={(event) =>
            onFieldChange("expenseDate", event.target.value)
          }
          aria-invalid={!!fieldErrors.expenseDate}
        />
        <FormMessage>{fieldErrors.expenseDate}</FormMessage>
      </FormField>

      {/* Error */}
      {submitError && <FormMessage>{submitError}</FormMessage>}

      {/* Actions */}
      <div className="flex gap-2">
        <Button
          type="button"
          variant="outline"
          className="flex-1"
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button type="submit" className="flex-1" disabled={submitting}>
          {submitting ? "Saving..." : "Save Correction"}
        </Button>
      </div>
    </Form>
  );
}
