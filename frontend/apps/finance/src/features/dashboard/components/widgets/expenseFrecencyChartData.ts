import type { ExpenseSuggestion } from "../../../expense-autocomplete/types";

export type ActiveRecencyBucket = Exclude<
  ExpenseSuggestion["recencyBucket"],
  "older"
>;

export type ActiveExpenseSuggestion = ExpenseSuggestion & {
  recencyBucket: ActiveRecencyBucket;
};

export interface ExpenseFrecencyChartDatum {
  name: string;
  frequency: number;
  recencyBucket: ActiveRecencyBucket;
  lastUsedAt: string;
  amount: number;
  currency: string;
  expenseType: string;
}

export const ACTIVE_RECENCY_BUCKETS: ActiveRecencyBucket[] = [
  "today",
  "last_7_days",
  "last_30_days",
];

export const RECENCY_LABELS: Record<ActiveRecencyBucket, string> = {
  today: "Today",
  last_7_days: "Last 7 days",
  last_30_days: "Last 30 days",
};

export const RECENCY_COLORS: Record<ActiveRecencyBucket, string> = {
  today: "var(--recency-today)",
  last_7_days: "var(--recency-last-7-days)",
  last_30_days: "var(--recency-last-30-days)",
};

export function isActiveExpenseSuggestion(
  suggestion: ExpenseSuggestion,
): suggestion is ActiveExpenseSuggestion {
  return suggestion.recencyBucket !== "older";
}
