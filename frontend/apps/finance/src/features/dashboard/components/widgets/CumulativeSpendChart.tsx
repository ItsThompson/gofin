import { formatCurrency, getCurrencySymbol } from "@gofin/core";
import type { CumulativeSpendPoint } from "@/types";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Line,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ComposedChart,
} from "recharts";

interface CumulativeSpendChartProps {
  data: CumulativeSpendPoint[];
  currency: string;
}

export function CumulativeSpendChart({ data, currency }: CumulativeSpendChartProps) {
  const chartData = data.map((point) => {
    const actual = point.actual / 100;
    const ideal = point.ideal / 100;
    const underBudget = actual <= ideal;
    return {
      day: point.day,
      actual,
      ideal,
      surplusBase: underBudget ? actual : undefined,
      surplusTop: underBudget ? ideal : undefined,
      deficitBase: !underBudget ? ideal : undefined,
      deficitTop: !underBudget ? actual : undefined,
    };
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Cumulative Spending</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={300}>
          <ComposedChart
            data={chartData}
            margin={{ top: 5, right: 20, left: 10, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis
              dataKey="day"
              label={{ value: "Day of Month", position: "insideBottom", offset: -5 }}
            />
            <YAxis
              tickFormatter={(value) => `${getCurrencySymbol(currency)}${value}`}
            />
            <Tooltip
              formatter={(value) => formatCurrency((value as number) * 100, currency)}
            />
            <Area
              type="monotone"
              dataKey="surplusTop"
              fill="rgba(34, 197, 94, 0.50)"
              stroke="none"
              name="Under Budget"
              connectNulls={false}
              isAnimationActive={false}
            />
            <Area
              type="monotone"
              dataKey="deficitTop"
              fill="rgba(239, 68, 68, 0.50)"
              stroke="none"
              name="Over Budget"
              connectNulls={false}
              isAnimationActive={false}
            />
            <Line
              type="monotone"
              dataKey="ideal"
              stroke="var(--muted-foreground)"
              strokeDasharray="5 5"
              strokeWidth={2}
              dot={false}
              name="Budget Pace"
            />
            <Line
              type="monotone"
              dataKey="actual"
              stroke="var(--primary)"
              strokeWidth={2}
              dot={false}
              name="Actual"
            />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
