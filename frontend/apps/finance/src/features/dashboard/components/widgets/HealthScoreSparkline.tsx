import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { HealthScoreTrendPoint } from "../../../../types";
import { BAND_COLOR_CLASS } from "./healthScoreDisplay";

interface HealthScoreSparklineProps {
  points: HealthScoreTrendPoint[];
}

/**
 * Compact line sparkline of recent monthly scores. Colored by the latest point's
 * band (via `currentColor`), mirroring the ScoreRing. Renders nothing with fewer
 * than two points, since a single point is not a trend.
 */
export function HealthScoreSparkline({ points }: HealthScoreSparklineProps) {
  if (points.length < 2) {
    return null;
  }

  const latestBand = points[points.length - 1].band;
  const chartData = points.map((point) => ({
    label: new Date(point.year, point.month - 1).toLocaleString("en-US", {
      month: "short",
    }),
    total: point.total,
  }));

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">
          Score trend
        </span>
        <span className="text-xs text-muted-foreground">
          Last {points.length} months
        </span>
      </div>
      <div className={`h-16 w-full ${BAND_COLOR_CLASS[latestBand]}`}>
        <ResponsiveContainer width="100%" height="100%">
          <LineChart
            data={chartData}
            margin={{ top: 4, right: 6, bottom: 0, left: 6 }}
          >
            <XAxis dataKey="label" hide />
            <YAxis hide domain={[0, 100]} />
            <Tooltip contentStyle={{ fontSize: "0.75rem", borderRadius: "0.5rem" }} />
            <Line
              name="Score"
              type="monotone"
              dataKey="total"
              stroke="currentColor"
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 3 }}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
