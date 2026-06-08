import { describe, it, expect } from "vitest";
import { insertCrossoverPoints } from "../insertCrossoverPoints";

describe("insertCrossoverPoints", () => {
  it("returns empty array for empty input", () => {
    expect(insertCrossoverPoints([])).toEqual([]);
  });

  it("returns single point unchanged", () => {
    const points = [{ day: 1, actual: 100, ideal: 200 }];
    expect(insertCrossoverPoints(points)).toEqual(points);
  });

  it("returns two points unchanged when no crossover occurs", () => {
    const points = [
      { day: 1, actual: 100, ideal: 200 },
      { day: 2, actual: 150, ideal: 250 },
    ];
    expect(insertCrossoverPoints(points)).toEqual(points);
  });

  it("inserts interpolated point when actual crosses above ideal", () => {
    const points = [
      { day: 1, actual: 100, ideal: 200 },
      { day: 2, actual: 300, ideal: 200 },
    ];
    const result = insertCrossoverPoints(points);

    expect(result).toHaveLength(3);
    // Crossover point: actual goes from 100 to 300 (+200), ideal stays at 200
    // At crossover: actual === ideal === 200
    // Linear interp: (200 - 100) / (300 - 100) = 0.5 of the interval
    // day = 1 + 0.5 * (2 - 1) = 1.5
    expect(result[1]).toEqual({
      day: 1.5,
      actual: 200,
      ideal: 200,
      isCrossover: true,
    });
  });

  it("inserts interpolated point when actual crosses below ideal", () => {
    const points = [
      { day: 5, actual: 500, ideal: 300 },
      { day: 10, actual: 200, ideal: 400 },
    ];
    const result = insertCrossoverPoints(points);

    expect(result).toHaveLength(3);
    // diff at day 5: 500 - 300 = 200 (actual > ideal)
    // diff at day 10: 200 - 400 = -200 (actual < ideal)
    // crossover fraction: 200 / (200 + 200) = 0.5
    // day = 5 + 0.5 * (10 - 5) = 7.5
    // actual at crossover: 500 + 0.5 * (200 - 500) = 350
    // ideal at crossover: 300 + 0.5 * (400 - 300) = 350
    expect(result[1]).toEqual({
      day: 7.5,
      actual: 350,
      ideal: 350,
      isCrossover: true,
    });
  });

  it("inserts multiple crossover points for multiple crossings", () => {
    const points = [
      { day: 1, actual: 100, ideal: 200 },
      { day: 2, actual: 300, ideal: 200 },
      { day: 3, actual: 100, ideal: 200 },
    ];
    const result = insertCrossoverPoints(points);

    // Two crossovers: between day 1-2 and day 2-3
    expect(result).toHaveLength(5);
    expect(result[1].isCrossover).toBe(true);
    expect(result[3].isCrossover).toBe(true);
  });

  it("does not insert crossover point when actual equals ideal at a data point", () => {
    const points = [
      { day: 1, actual: 100, ideal: 200 },
      { day: 2, actual: 200, ideal: 200 },
      { day: 3, actual: 300, ideal: 200 },
    ];
    const result = insertCrossoverPoints(points);

    // At day 2, actual === ideal exactly. No synthetic point needed between 1-2
    // because the sign doesn't cross (it goes from negative to zero to positive).
    // Between day 1 and 2: diff goes from -100 to 0 (no sign change, zero is not crossing)
    // Between day 2 and 3: diff goes from 0 to +100 (no sign change)
    expect(result).toHaveLength(3);
    expect(result).toEqual(points);
  });

  it("preserves original points without mutation", () => {
    const points = [
      { day: 1, actual: 100, ideal: 200 },
      { day: 2, actual: 300, ideal: 200 },
    ];
    const original = JSON.parse(JSON.stringify(points));
    insertCrossoverPoints(points);
    expect(points).toEqual(original);
  });
});
