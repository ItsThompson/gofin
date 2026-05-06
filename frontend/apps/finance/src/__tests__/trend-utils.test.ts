import { describe, it, expect } from "vitest";
import {
  computeCategoryPercentages,
} from "@/lib/trend-utils";

describe("computeCategoryPercentages", () => {
  it("computes percentages correctly for normal spending", () => {
    const result = computeCategoryPercentages(5000, 3000, 1500);
    expect(result.essentials).toBeCloseTo(52.6, 1);
    expect(result.desires).toBeCloseTo(31.6, 1);
    expect(result.savings).toBeCloseTo(15.8, 1);
  });

  it("returns zeros when totalSpent is zero", () => {
    const result = computeCategoryPercentages(0, 0, 0);
    expect(result.essentials).toBe(0);
    expect(result.desires).toBe(0);
    expect(result.savings).toBe(0);
  });

  it("handles single category spending", () => {
    const result = computeCategoryPercentages(10000, 0, 0);
    expect(result.essentials).toBe(100);
    expect(result.desires).toBe(0);
    expect(result.savings).toBe(0);
  });

  it("rounds to one decimal place", () => {
    // 3333/10000 = 33.33% → 33.3, 3334/10000 = 33.34% → 33.3
    const result = computeCategoryPercentages(3333, 3333, 3334);
    expect(result.essentials).toBe(33.3);
    expect(result.desires).toBe(33.3);
    expect(result.savings).toBe(33.3);
  });

  it("categories approximately sum to 100% for normal spending", () => {
    const result = computeCategoryPercentages(8000, 5000, 2000);
    const sum = result.essentials + result.desires + result.savings;
    expect(sum).toBeCloseTo(100, 0);
  });

  it("handles very small amounts without precision issues", () => {
    const result = computeCategoryPercentages(1, 1, 1);
    expect(result.essentials).toBe(33.3);
    expect(result.desires).toBe(33.3);
    expect(result.savings).toBe(33.3);
  });
});
