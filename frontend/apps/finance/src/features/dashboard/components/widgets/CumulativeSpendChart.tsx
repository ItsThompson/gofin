import { formatCurrency, getCurrencySymbol } from "@gofin/core";
import type { CumulativeSpendPoint } from "../../../../types";
import { insertCrossoverPoints } from "../../../../lib/insertCrossoverPoints";
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
  ReferenceDot,
} from "recharts";

interface CumulativeSpendChartProps {
  data: CumulativeSpendPoint[];
  currency: string;
}

export function CumulativeSpendChart({ data, currency }: CumulativeSpendChartProps) {
  const currentDay = new Date().getDate();

  // Convert from cents to dollars and insert crossover interpolation points
  const basePoints = data.map((point) => ({
    day: point.day,
    actual: point.actual / 100,
    ideal: point.ideal / 100,
  }));

  const interpolatedPoints = insertCrossoverPoints(basePoints);

  const chartData = interpolatedPoints.map((point) => {
    const underBudget = point.actual <= point.ideal;

    if (point.isCrossover) {
      // At crossover, define both areas to terminate/start seamlessly
      return {
        day: point.day,
        actual: point.actual,
        ideal: point.ideal,
        surplusTop: point.actual,
        deficitTop: point.actual,
      };
    }

    return {
      day: point.day,
      actual: point.actual,
      ideal: point.ideal,
      surplusTop: underBudget ? point.ideal : undefined,
      deficitTop: !underBudget ? point.actual : undefined,
    };
  });

  const todayPoint = chartData.find(
    (point) => point.day === currentDay && point.actual != null,
  );

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
            {todayPoint && (
              <ReferenceDot
                x={todayPoint.day}
                y={todayPoint.actual}
                shape={(props: { cx?: number; cy?: number }) => {
                  const { cx = 0, cy = 0 } = props;
                  const size = 6;
                  return (
                    <polygon
                      points={`${cx},${cy - size} ${cx + size},${cy} ${cx},${cy + size} ${cx - size},${cy}`}
                      fill="var(--primary)"
                      stroke="var(--background)"
                      strokeWidth={1.5}
                    />
                  );
                }}
              />
            )}
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
