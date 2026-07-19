import {
  RadialBarChart,
  RadialBar,
  PolarAngleAxis,
  ResponsiveContainer,
} from "recharts";
import type { HealthBand } from "../../../../types";
import { BAND_COLOR_CLASS, BAND_LABEL } from "./healthScoreDisplay";

interface ScoreRingProps {
  /** Total score, 0-100. */
  total: number;
  band: HealthBand;
}

/**
 * Open-bottom gauge arc for the health-score total. recharts cannot center text
 * inside the arc, so the number and band label are an absolutely-positioned
 * overlay. The arc and number take the band color via `currentColor`.
 */
export function ScoreRing({ total, band }: ScoreRingProps) {
  const data = [{ value: total }];

  return (
    <div className={`relative mx-auto h-40 w-40 ${BAND_COLOR_CLASS[band]}`}>
      <ResponsiveContainer width="100%" height="100%">
        <RadialBarChart
          data={data}
          startAngle={225}
          endAngle={-45}
          innerRadius="72%"
          outerRadius="100%"
          barSize={12}
        >
          <PolarAngleAxis
            type="number"
            domain={[0, 100]}
            angleAxisId={0}
            tick={false}
          />
          <RadialBar
            background={{ fill: "var(--color-muted)" }}
            dataKey="value"
            cornerRadius={8}
            angleAxisId={0}
            fill="currentColor"
            isAnimationActive={false}
          />
        </RadialBarChart>
      </ResponsiveContainer>
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-4xl font-bold leading-none">{total}</span>
        <span className="mt-1 text-xs font-medium text-muted-foreground">
          {BAND_LABEL[band]}
        </span>
      </div>
    </div>
  );
}
