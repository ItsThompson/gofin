import type { ExpenseSuggestion } from "../../../expense-autocomplete/types";

export interface ExpenseFrecencyChartDatum {
  name: string;
  frequency: number;
  recencyBucket: ExpenseSuggestion["recencyBucket"];
  lastUsedAt: string;
  amount: number;
  currency: string;
  expenseType: string;
}

export const RECENCY_LABELS: Record<ExpenseSuggestion["recencyBucket"], string> = {
  today: "Today",
  last_7_days: "Last 7 days",
  last_30_days: "Last 30 days",
  older: "Older",
};

export const RECENCY_COLORS: Record<ExpenseSuggestion["recencyBucket"], string> = {
  today: "var(--primary)",
  last_7_days: "var(--chart-2)",
  last_30_days: "var(--chart-3)",
  older: "var(--muted-foreground)",
};
