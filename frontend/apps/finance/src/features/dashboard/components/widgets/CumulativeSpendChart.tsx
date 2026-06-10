import { getCurrencySymbol } from "@gofin/core";
import type { CumulativeSpendPoint } from "../../../../types";
import { insertCrossoverPoints } from "../../../../lib/insertCrossoverPoints";
import {
  tooltipFormatter,
  tooltipLabelFormatter,
} from "./cumulative-spend-chart-utils";
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
      // At crossover, both areas collapse to zero height for seamless transition
      return {
        day: point.day,
        actual: point.actual,
        ideal: point.ideal,
        surplus: [point.actual, point.actual] as [number, number],
        deficit: [point.actual, point.actual] as [number, number],
      };
    }

    return {
      day: point.day,
      actual: point.actual,
      ideal: point.ideal,
      surplus: underBudget
        ? ([point.actual, point.ideal] as [number, number])
        : undefined,
      deficit: !underBudget
        ? ([point.ideal, point.actual] as [number, number])
        : undefined,
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
              type="number"
              domain={[1, basePoints.length]}
              ticks={basePoints.map((p) => p.day)}
              allowDecimals={false}
              label={{ value: "Day of Month", position: "insideBottom", offset: -5 }}
            />
            <YAxis
              tickFormatter={(value) => `${getCurrencySymbol(currency)}${value}`}
            />
            <Tooltip
              formatter={(value, name) => tooltipFormatter(value, name as string, currency)}
              labelFormatter={(label) => tooltipLabelFormatter(label)}
            />
            <Area
              type="monotone"
              dataKey="surplus"
              fill="rgba(34, 197, 94, 0.50)"
              stroke="none"
              name="Under Budget"
              connectNulls={false}
              isAnimationActive={false}
            />
            <Area
              type="monotone"
              dataKey="deficit"
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
