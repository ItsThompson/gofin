import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router";
import {
  apiClient,
  ApiRequestError,
} from "@gofin/api";
import { getCurrencySymbol } from "@gofin/core";
import type {
  ExpenseResponse,
  CreateExpenseRequest,
  CreateProRataRequest,
  ProRataResponse,
  Tag,
  TagListResponse,
} from "@/types";
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
import type { FinancePageProps } from "../types";

/** Valid expense types matching the backend enum. */
const EXPENSE_TYPES = ["essentials", "desires", "savings"] as const;
type ExpenseType = (typeof EXPENSE_TYPES)[number];

function todayISO(): string {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

/**
 * New expense form page. Allows users to log a standard expense.
 *
 * Amount is entered in dollars (with decimals) and converted to cents
 * (integer) before submission. On success, redirects to /dashboard.
 *
 * Exported via Module Federation for the shell to load dynamically.
 */
export function NewExpensePage({ user }: FinancePageProps) {
  const navigate = useNavigate();
  const currencySymbol = getCurrencySymbol(user.currency);

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const [tags, setTags] = useState<Tag[]>([]);
  const [tagsLoading, setTagsLoading] = useState(true);
  const [name, setName] = useState("");
  const [amountDollars, setAmountDollars] = useState("");
  const [expenseType, setExpenseType] = useState<ExpenseType>("essentials");
  const [tagId, setTagId] = useState("");
  const [expenseDate, setExpenseDate] = useState(todayISO());
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [isProRata, setIsProRata] = useState(false);
  const [proRataMonths, setProRataMonths] = useState("");

  useEffect(() => {
    async function fetchTags() {
      try {
        const response = await apiClient<TagListResponse>("/api/finance/tags");
        setTags(response.tags);
        if (response.tags.length > 0) {
          setTagId(response.tags[0].id);
        }
      } catch {
        // If tags fail to load, form will show empty dropdown
      } finally {
        setTagsLoading(false);
      }
    }
    fetchTags();
  }, []);

  function validate(): Record<string, string> {
    const errors: Record<string, string> = {};

    if (!name.trim()) {
      errors.name = "Name is required";
    }

    const parsedAmount = parseFloat(amountDollars);
    if (!amountDollars || isNaN(parsedAmount) || parsedAmount <= 0) {
      errors.amount = "Amount must be greater than 0";
    }

    if (!expenseDate) {
      errors.expenseDate = "Date is required";
    }

    if (!tagId) {
      errors.tagId = "Tag is required";
    }

    if (isProRata) {
      const months = parseInt(proRataMonths, 10);
      if (!proRataMonths || isNaN(months) || months < 2) {
        errors.proRataMonths = "Must be at least 2 months";
      }
    }

    return errors;
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});

    const errors = validate();
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    const amountCents = Math.round(parseFloat(amountDollars) * 100);

    setSubmitting(true);
    try {
      if (isProRata) {
        const body: CreateProRataRequest = {
          name: name.trim(),
          totalAmount: amountCents,
          currency: user.currency,
          expenseType,
          tagId,
          expenseDate,
          months: parseInt(proRataMonths, 10),
        };
        await apiClient<ProRataResponse>("/api/finance/prorata", {
          method: "POST",
          body: JSON.stringify(body),
        });
      } else {
        const body: CreateExpenseRequest = {
          name: name.trim(),
          amount: amountCents,
          currency: user.currency,
          expenseType,
          tagId,
          expenseDate,
          periodYear: currentYear,
          periodMonth: currentMonth,
        };
        await apiClient<ExpenseResponse>("/api/expenses", {
          method: "POST",
          body: JSON.stringify(body),
        });
      }
      navigate("/dashboard");
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
          <Form onSubmit={handleSubmit}>
            {/* Name */}
            <FormField>
              <FormLabel htmlFor="expense-name">Name</FormLabel>
              <Input
                id="expense-name"
                type="text"
                placeholder="e.g. Grocery shopping"
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
                      checked={expenseType === type}
                      onChange={() => setExpenseType(type)}
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
                value={tagId}
                onChange={(event) => {
                  setTagId(event.target.value);
                  setFieldErrors((prev) => ({ ...prev, tagId: "" }));
                }}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                aria-invalid={!!fieldErrors.tagId}
              >
                {tagsLoading ? (
                  <option value="">Loading tags...</option>
                ) : (
                  tags.map((tag) => (
                    <option key={tag.id} value={tag.id}>
                      {tag.name}
                    </option>
                  ))
                )}
              </select>
              <FormMessage>{fieldErrors.tagId}</FormMessage>
            </FormField>

            {/* Date */}
            <FormField>
              <FormLabel htmlFor="expense-date">Date</FormLabel>
              <Input
                id="expense-date"
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

            {/* Pro-rata Toggle */}
            <FormField>
              <label className="flex cursor-pointer items-center gap-2">
                <input
                  type="checkbox"
                  checked={isProRata}
                  onChange={(event) => {
                    setIsProRata(event.target.checked);
                    if (!event.target.checked) {
                      setProRataMonths("");
                      setFieldErrors((prev) => ({ ...prev, proRataMonths: "" }));
                    }
                  }}
                  className="size-4 accent-primary"
                />
                <span className="text-sm font-medium">Spread across months</span>
              </label>
            </FormField>

            {/* Pro-rata Months */}
            {isProRata && (
              <FormField>
                <FormLabel htmlFor="pro-rata-months">Number of months</FormLabel>
                <Input
                  id="pro-rata-months"
                  type="number"
                  min="2"
                  step="1"
                  placeholder="e.g. 3"
                  value={proRataMonths}
                  onChange={(event) => {
                    setProRataMonths(event.target.value);
                    setFieldErrors((prev) => ({ ...prev, proRataMonths: "" }));
                  }}
                  aria-invalid={!!fieldErrors.proRataMonths}
                />
                <FormMessage>{fieldErrors.proRataMonths}</FormMessage>
              </FormField>
            )}

            {/* Error Message */}
            {error && <FormMessage>{error}</FormMessage>}

            {/* Submit */}
            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "Saving..." : "Log Expense"}
            </Button>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
