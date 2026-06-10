import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  CumulativeSpendChart,
} from "../components/widgets/CumulativeSpendChart";
import {
  tooltipFormatter,
  tooltipLabelFormatter,
} from "../components/widgets/cumulative-spend-chart-utils";

// ResponsiveContainer won't render children without real DOM dimensions.
// Mock it to render children directly so chart internals execute.
vi.mock("recharts", async (importOriginal) => {
  const actual = await importOriginal<typeof import("recharts")>();
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 800, height: 300 }}>{children}</div>
    ),
  };
});

// Data that produces a crossover: actual starts above ideal, then drops below
const testData = [
  { day: 1, actual: 20000, ideal: 10000 },
  { day: 2, actual: 35000, ideal: 20000 },
  { day: 3, actual: 40000, ideal: 30000 },
  { day: 4, actual: 42000, ideal: 40000 },
  { day: 5, actual: 44000, ideal: 50000 },
  { day: 6, actual: 46000, ideal: 60000 },
  { day: 7, actual: 48000, ideal: 70000 },
];

describe("CumulativeSpendChart", () => {
  it("renders chart card with title", () => {
    render(<CumulativeSpendChart data={testData} currency="GBP" />);
    expect(screen.getByText("Cumulative Spending")).toBeInTheDocument();
  });

  it("renders with empty data without crashing", () => {
    render(<CumulativeSpendChart data={[]} currency="USD" />);
    expect(screen.getByText("Cumulative Spending")).toBeInTheDocument();
  });
});

describe("tooltipFormatter", () => {
  it("returns null for array values (range area tuples)", () => {
    expect(tooltipFormatter([100, 200], "surplus", "GBP")).toBeNull();
  });

  it("formats scalar values as currency", () => {
    const result = tooltipFormatter(150, "Actual", "GBP");
    expect(result).not.toBeNull();
    expect(result![0]).toContain("£");
    expect(result![1]).toBe("Actual");
  });
});

describe("tooltipLabelFormatter", () => {
  it("formats integer day labels", () => {
    expect(tooltipLabelFormatter(5)).toBe("Day 5");
    expect(tooltipLabelFormatter(17)).toBe("Day 17");
  });

  it("returns empty string for fractional crossover values", () => {
    expect(tooltipLabelFormatter(17.16)).toBe("");
    expect(tooltipLabelFormatter(4.5)).toBe("");
  });
});
