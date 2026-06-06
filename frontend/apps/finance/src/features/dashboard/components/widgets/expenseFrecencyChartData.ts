import type { ExpenseSuggestion } from "../../../expense-autocomplete/types";

export type ActiveRecencyBucket = Exclude<
  ExpenseSuggestion["recencyBucket"],
  "older"
>;

export interface ExpenseFrecencyChartDatum {
  name: string;
  frequency: number;
  recencyBucket: ActiveRecencyBucket;
  lastUsedAt: string;
  amount: number;
  currency: string;
  expenseType: string;
}

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

export function isActiveRecencyBucket(
  bucket: ExpenseSuggestion["recencyBucket"],
): bucket is ActiveRecencyBucket {
  return bucket !== "older";
}
