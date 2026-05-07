import {
  ComposedChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import { formatCurrency, getCurrencySymbol } from "@gofin/core";
import type { TrendPoint } from "../../../../types";
import { MONTH_LABELS } from "../../../../lib/trend-utils";

interface SpendingTrendChartProps {
  data: TrendPoint[];
  currency: string;
}

export function SpendingTrendChart({ data, currency }: SpendingTrendChartProps) {
  const chartData = data.map((point) => ({
    label: `${MONTH_LABELS[point.month]} '${String(point.year).slice(2)}`,
    spending: point.totalSpent / 100,
    budget: point.budgetAmount / 100,
  }));

  const currencySymbol = getCurrencySymbol(currency);

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base">Monthly Spending</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={260}>
          <ComposedChart
            data={chartData}
            margin={{ top: 5, right: 20, left: 10, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="label" tick={{ fontSize: 12 }} />
            <YAxis
              tickFormatter={(value: number) => `${currencySymbol}${value}`}
            />
            <Tooltip
              formatter={(value, name) => [
                formatCurrency((value as number) * 100, currency),
                (name as string) === "spending" ? "Spent" : "Budget",
              ]}
            />
            <Line
              type="monotone"
              dataKey="budget"
              stroke="var(--color-muted-foreground)"
              strokeDasharray="5 5"
              strokeWidth={2}
              dot={false}
              name="budget"
            />
            <Line
              type="monotone"
              dataKey="spending"
              stroke="var(--color-primary)"
              strokeWidth={2}
              dot={{ r: 3 }}
              name="spending"
            />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
