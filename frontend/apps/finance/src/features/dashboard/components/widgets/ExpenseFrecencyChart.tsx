import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatCurrency } from "@gofin/core";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import type { ExpenseSuggestion } from "../../../expense-autocomplete/types";
import type { ExpenseFrecencyDataState } from "../../hooks/useExpenseFrecencyData";

interface ExpenseFrecencyChartProps extends ExpenseFrecencyDataState {}

interface ChartDatum {
  name: string;
  frequency: number;
  recencyBucket: ExpenseSuggestion["recencyBucket"];
  lastUsedAt: string;
  amount: number;
  currency: string;
  expenseType: string;
}

const RECENCY_LABELS: Record<ExpenseSuggestion["recencyBucket"], string> = {
  today: "Today",
  last_7_days: "Last 7 days",
  last_30_days: "Last 30 days",
  older: "Older",
};

const RECENCY_COLORS: Record<ExpenseSuggestion["recencyBucket"], string> = {
  today: "var(--primary)",
  last_7_days: "var(--chart-2)",
  last_30_days: "var(--chart-3)",
  older: "var(--muted-foreground)",
};

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

interface ExpenseFrecencyTooltipProps {
  active?: boolean;
  payload?: Array<{ payload: ChartDatum }>;
}

function ExpenseFrecencyTooltip({ active, payload }: ExpenseFrecencyTooltipProps) {
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

export function ExpenseFrecencyChart({
  status,
  suggestions,
}: ExpenseFrecencyChartProps) {
  const chartData: ChartDatum[] = suggestions.map((suggestion) => ({
    name: suggestion.name,
    frequency: suggestion.frequency,
    recencyBucket: suggestion.recencyBucket,
    lastUsedAt: suggestion.lastUsedAt,
    amount: suggestion.amount,
    currency: suggestion.currency,
    expenseType: suggestion.expenseType,
  }));

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Repeated Expenses</CardTitle>
      </CardHeader>
      <CardContent>
        {status === "loading" && (
          <p className="text-sm text-muted-foreground">Loading repeated expenses...</p>
        )}
        {status === "error" && (
          <p className="text-sm text-muted-foreground">
            Repeated expenses are unavailable right now.
          </p>
        )}
        {status === "empty" && (
          <p className="text-sm text-muted-foreground">
            Not enough expense history yet to show repeated expenses.
          </p>
        )}
        {status === "success" && (
          <>
            <p className="mb-4 text-sm text-muted-foreground">
              Frequency shows how often you have logged each expense. Color shows recency.
            </p>
            <div className="mb-3 flex flex-wrap gap-3 text-xs text-muted-foreground" aria-label="Recency legend">
              {Object.entries(RECENCY_LABELS).map(([bucket, label]) => (
                <span key={bucket} className="inline-flex items-center gap-1">
                  <span
                    className="size-2 rounded-full"
                    style={{ backgroundColor: RECENCY_COLORS[bucket as ExpenseSuggestion["recencyBucket"]] }}
                  />
                  {label}
                </span>
              ))}
            </div>
            <ResponsiveContainer width="100%" height={Math.max(240, chartData.length * 42)}>
              <BarChart
                data={chartData}
                layout="vertical"
                margin={{ top: 0, right: 24, left: 10, bottom: 0 }}
              >
                <CartesianGrid strokeDasharray="3 3" horizontal={false} />
                <XAxis type="number" dataKey="frequency" allowDecimals={false} />
                <YAxis type="category" dataKey="name" width={120} tick={{ fontSize: 12 }} />
                <Tooltip content={<ExpenseFrecencyTooltip />} />
                <Bar dataKey="frequency" radius={[0, 4, 4, 0]}>
                  {chartData.map((datum) => (
                    <Cell key={datum.name} fill={RECENCY_COLORS[datum.recencyBucket]} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
            <ul className="mt-4 space-y-2 text-sm" aria-label="Repeated expense details">
              {chartData.map((datum) => (
                <li key={datum.name} className="flex flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
                  <span className="font-medium text-foreground">{datum.name}</span>
                  <span>Frequency: {datum.frequency}</span>
                  <span>Recency: {RECENCY_LABELS[datum.recencyBucket]}</span>
                </li>
              ))}
            </ul>
          </>
        )}
      </CardContent>
    </Card>
  );
}
