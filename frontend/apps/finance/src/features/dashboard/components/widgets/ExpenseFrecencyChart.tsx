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
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import type { ExpenseFrecencyDataState } from "../../hooks/useExpenseFrecencyData";
import { ExpenseFrecencyTooltip } from "./ExpenseFrecencyTooltip";
import {
  RECENCY_COLORS,
  RECENCY_LABELS,
  isActiveRecencyBucket,
} from "./expenseFrecencyChartData";
import type { ExpenseFrecencyChartDatum } from "./expenseFrecencyChartData";

export function ExpenseFrecencyChart({
  status,
  suggestions,
}: ExpenseFrecencyDataState) {
  const chartData: ExpenseFrecencyChartDatum[] = suggestions.flatMap(
    (suggestion) => {
      if (!isActiveRecencyBucket(suggestion.recencyBucket)) return [];

      return [
        {
          name: suggestion.name,
          frequency: suggestion.frequency,
          recencyBucket: suggestion.recencyBucket,
          lastUsedAt: suggestion.lastUsedAt,
          amount: suggestion.amount,
          currency: suggestion.currency,
          expenseType: suggestion.expenseType,
        },
      ];
    },
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Repeated Expenses</CardTitle>
      </CardHeader>
      <CardContent>
        {status === "loading" && (
          <p className="text-sm text-muted-foreground">
            Loading repeated expenses...
          </p>
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
              Frequency shows how often you have logged each expense. Color
              shows recency.
            </p>
            <div
              className="mb-3 flex flex-wrap gap-3 text-xs text-muted-foreground"
              aria-label="Recency legend"
            >
              {Object.entries(RECENCY_LABELS).map(([bucket, label]) => (
                <span key={bucket} className="inline-flex items-center gap-1">
                  <span
                    className="size-2 rounded-full"
                    style={{
                      backgroundColor:
                        RECENCY_COLORS[bucket as keyof typeof RECENCY_COLORS],
                    }}
                  />
                  {label}
                </span>
              ))}
            </div>
            <ResponsiveContainer
              width="100%"
              height={Math.max(240, chartData.length * 42)}
            >
              <BarChart
                data={chartData}
                layout="vertical"
                margin={{ top: 0, right: 24, left: 10, bottom: 0 }}
              >
                <CartesianGrid strokeDasharray="3 3" horizontal={false} />
                <XAxis
                  type="number"
                  dataKey="frequency"
                  allowDecimals={false}
                />
                <YAxis
                  type="category"
                  dataKey="name"
                  width={120}
                  tick={{ fontSize: 12 }}
                />
                <Tooltip content={<ExpenseFrecencyTooltip />} />
                <Bar dataKey="frequency" radius={[0, 4, 4, 0]}>
                  {chartData.map((datum) => (
                    <Cell
                      key={datum.name}
                      fill={RECENCY_COLORS[datum.recencyBucket]}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </>
        )}
      </CardContent>
    </Card>
  );
}
