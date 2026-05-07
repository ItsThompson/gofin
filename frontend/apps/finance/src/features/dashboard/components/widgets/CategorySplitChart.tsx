import {
  ComposedChart,
  Bar,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import type { TrendPoint } from "@/types";
import { computeCategoryPercentages, MONTH_LABELS } from "../../../../lib/trend-utils";

interface CategorySplitChartProps {
  data: TrendPoint[];
}

export function CategorySplitChart({ data }: CategorySplitChartProps) {
  const chartData = data.map((point) => {
    const percentages = computeCategoryPercentages(
      point.essentialsSpent,
      point.desiresSpent,
      point.savingsSpent,
    );

    return {
      label: `${MONTH_LABELS[point.month]} '${String(point.year).slice(2)}`,
      essentialsActual: percentages.essentials,
      desiresActual: percentages.desires,
      savingsActual: percentages.savings,
      essentialsBudget: point.essentialsPercent,
      desiresBudget: point.desiresPercent,
      savingsBudget: point.savingsPercent,
    };
  });

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base">Category Split</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={280}>
          <ComposedChart
            data={chartData}
            margin={{ top: 5, right: 20, left: 10, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="label" tick={{ fontSize: 12 }} />
            <YAxis
              tickFormatter={(value: number) => `${value}%`}
              domain={[0, 100]}
            />
            <Tooltip
              formatter={(value, name) => {
                const label = (name as string).replace("Actual", "").replace("Budget", " target");
                return [`${value}%`, label];
              }}
            />
            <Legend />
            {/* Bars: actual spending percentages */}
            <Bar
              dataKey="essentialsActual"
              fill="var(--color-essentials)"
              name="Essentials"
              radius={[2, 2, 0, 0]}
            />
            <Bar
              dataKey="desiresActual"
              fill="var(--color-desires)"
              name="Desires"
              radius={[2, 2, 0, 0]}
            />
            <Bar
              dataKey="savingsActual"
              fill="var(--color-savings)"
              name="Savings"
              radius={[2, 2, 0, 0]}
            />
            {/* Lines: budgeted percentages */}
            <Line
              type="monotone"
              dataKey="essentialsBudget"
              stroke="var(--color-essentials)"
              strokeDasharray="4 4"
              strokeWidth={2}
              strokeOpacity={0.6}
              dot={false}
              name="Essentials target"
            />
            <Line
              type="monotone"
              dataKey="desiresBudget"
              stroke="var(--color-desires)"
              strokeDasharray="4 4"
              strokeWidth={2}
              strokeOpacity={0.6}
              dot={false}
              name="Desires target"
            />
            <Line
              type="monotone"
              dataKey="savingsBudget"
              stroke="var(--color-savings)"
              strokeDasharray="4 4"
              strokeWidth={2}
              strokeOpacity={0.6}
              dot={false}
              name="Savings target"
            />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
