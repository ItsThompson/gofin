import { useState, type FormEvent } from "react";
import { getCurrencySymbol } from "@gofin/core";
import type { Expense, Tag, CorrectExpenseRequest } from "@/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";

const EXPENSE_TYPES = ["essentials", "desires", "savings"] as const;
type ExpenseType = (typeof EXPENSE_TYPES)[number];

interface CorrectionFormProps {
  expense: Expense;
  currency: string;
  tags: Tag[];
  onCancel: () => void;
  onSubmit: (form: CorrectExpenseRequest) => void;
  submitting: boolean;
  submitError: string | null;
}

export function CorrectionForm({
  expense,
  currency,
  tags,
  onCancel,
  onSubmit,
  submitting,
  submitError,
}: CorrectionFormProps) {
  const currencySymbol = getCurrencySymbol(currency);

  const [name, setName] = useState(expense.name);
  const [amountDollars, setAmountDollars] = useState(
    (expense.amount / 100).toFixed(2),
  );
  const [expenseType, setExpenseType] = useState<ExpenseType>(
    expense.expenseType,
  );
  const [tagId, setTagId] = useState(expense.tagId);
  const [expenseDate, setExpenseDate] = useState(expense.expenseDate);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  function validate(): Record<string, string> {
    const errors: Record<string, string> = {};
    if (!name.trim()) errors.name = "Name is required";
    const parsed = parseFloat(amountDollars);
    if (!amountDollars || isNaN(parsed) || parsed <= 0) {
      errors.amount = "Amount must be greater than 0";
    }
    if (!expenseDate) errors.expenseDate = "Date is required";
    if (!tagId) errors.tagId = "Tag is required";
    return errors;
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setFieldErrors({});

    const errors = validate();
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    const amountCents = Math.round(parseFloat(amountDollars) * 100);

    const body: CorrectExpenseRequest = {
      name: name.trim(),
      amount: amountCents,
      expenseType,
      tagId,
      expenseDate,
    };

    onSubmit(body);
  }

  return (
    <Form onSubmit={handleSubmit}>
      {/* Name */}
      <FormField>
        <FormLabel htmlFor="correct-name">Name</FormLabel>
        <Input
          id="correct-name"
          type="text"
          value={name}
          onChange={(event) => {
            setName(event.target.value);
            setFieldErrors((prev) => ({ ...prev, name: "" }));
          }}
          aria-invalid={!!fieldErrors.name}
        />
        <FormMessage>{fieldErrors.name}</FormMessage>
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
            value={amountDollars}
            onChange={(event) => {
              setAmountDollars(event.target.value);
              setFieldErrors((prev) => ({ ...prev, amount: "" }));
            }}
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
                checked={expenseType === type}
                onChange={() => setExpenseType(type)}
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
          value={tagId}
          onChange={(event) => {
            setTagId(event.target.value);
            setFieldErrors((prev) => ({ ...prev, tagId: "" }));
          }}
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
          value={expenseDate}
          onChange={(event) => {
            setExpenseDate(event.target.value);
            setFieldErrors((prev) => ({ ...prev, expenseDate: "" }));
          }}
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
