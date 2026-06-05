import { formatCurrency } from "@gofin/core";
import { RECENCY_LABELS } from "./expenseFrecencyChartData";
import type { ExpenseFrecencyChartDatum } from "./expenseFrecencyChartData";

export interface ExpenseFrecencyTooltipProps {
  active?: boolean;
  payload?: Array<{ payload: ExpenseFrecencyChartDatum }>;
}

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function ExpenseFrecencyTooltip({
  active,
  payload,
}: ExpenseFrecencyTooltipProps) {
  if (!active || !payload?.length) return null;

  const datum = payload[0].payload;

  return (
    <div className="rounded-md border bg-background p-3 text-sm shadow-sm">
      <p className="font-medium">{datum.name}</p>
      <p>Frequency: {datum.frequency}</p>
      <p>Recency: {RECENCY_LABELS[datum.recencyBucket]}</p>
      <p>Last used: {formatDate(datum.lastUsedAt)}</p>
      <p>Latest amount: {formatCurrency(datum.amount, datum.currency)}</p>
      <p>Type: {datum.expenseType}</p>
    </div>
  );
}
