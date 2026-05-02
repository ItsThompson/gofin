import { describe, it, expect } from "vitest";
import { getRemainingColor } from "@/lib/budget-utils";

describe("getRemainingColor", () => {
  it("returns empty string when budget is $0", () => {
    expect(getRemainingColor(0, 0)).toBe("");
  });

  it("returns green when remaining is > 30% of budget", () => {
    // 50% remaining
    const result = getRemainingColor(100000, 50000);
    expect(result).toContain("text-green");
  });

  it("returns green at exactly 31% remaining", () => {
    const result = getRemainingColor(100000, 31000);
    expect(result).toContain("text-green");
  });

  it("returns yellow at exactly 30% remaining", () => {
    const result = getRemainingColor(100000, 30000);
    expect(result).toContain("text-yellow");
  });

  it("returns yellow at 20% remaining", () => {
    const result = getRemainingColor(100000, 20000);
    expect(result).toContain("text-yellow");
  });

  it("returns yellow at exactly 10% remaining", () => {
    const result = getRemainingColor(100000, 10000);
    expect(result).toContain("text-yellow");
  });

  it("returns red at 9% remaining", () => {
    const result = getRemainingColor(100000, 9000);
    expect(result).toContain("text-red");
  });

  it("returns red at 0% remaining (fully spent)", () => {
    const result = getRemainingColor(100000, 0);
    expect(result).toContain("text-red");
  });

  it("returns red when overspent (negative remaining)", () => {
    const result = getRemainingColor(100000, -5000);
    expect(result).toContain("text-red");
  });

  it("returns green at 100% remaining (nothing spent)", () => {
    const result = getRemainingColor(300000, 300000);
    expect(result).toContain("text-green");
  });
});
