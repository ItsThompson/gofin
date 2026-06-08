export interface ChartPoint {
  day: number;
  actual: number;
  ideal: number;
  isCrossover?: boolean;
}

/**
 * Insert synthetic interpolated data points at crossover coordinates where
 * the "actual" line crosses the "ideal" line. This eliminates shading gaps
 * in area charts that use separate fill regions for above/below states.
 *
 * For consecutive points where (actual - ideal) changes sign, the function
 * linear-interpolates the exact fractional day where actual === ideal and
 * inserts a synthetic point marked with `isCrossover: true`.
 */
export function insertCrossoverPoints(points: ChartPoint[]): ChartPoint[] {
  if (points.length <= 1) return points;

  const result: ChartPoint[] = [];

  for (let i = 0; i < points.length; i++) {
    const current = points[i];

    if (i > 0) {
      const prev = points[i - 1];
      const prevDiff = prev.actual - prev.ideal;
      const currDiff = current.actual - current.ideal;

      // Sign change means a crossover occurred between these two points.
      // Exclude zero values: if either endpoint is exactly on the line,
      // no synthetic point is needed.
      if (prevDiff !== 0 && currDiff !== 0 && prevDiff * currDiff < 0) {
        // Linear interpolation: find fraction t where diff = 0
        const t = Math.abs(prevDiff) / (Math.abs(prevDiff) + Math.abs(currDiff));
        const crossoverDay = prev.day + t * (current.day - prev.day);
        const crossoverValue = prev.actual + t * (current.actual - prev.actual);

        result.push({
          day: crossoverDay,
          actual: crossoverValue,
          ideal: crossoverValue,
          isCrossover: true,
        });
      }
    }

    result.push(current);
  }

  return result;
}
