import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TrendsSection } from "@/features/dashboard";
import type { TrendPoint } from "@gofin/core";

const mockTrendData: TrendPoint[] = [
  {
    year: 2025,
    month: 12,
    totalSpent: 250000,
    budgetAmount: 300000,
    essentialsSpent: 125000,
    desiresSpent: 75000,
    savingsSpent: 50000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
  },
  {
    year: 2026,
    month: 1,
    totalSpent: 280000,
    budgetAmount: 310000,
    essentialsSpent: 140000,
    desiresSpent: 84000,
    savingsSpent: 56000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
  },
  {
    year: 2026,
    month: 2,
    totalSpent: 0,
    budgetAmount: 300000,
    essentialsSpent: 0,
    desiresSpent: 0,
    savingsSpent: 0,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
  },
  {
    year: 2026,
    month: 3,
    totalSpent: 190000,
    budgetAmount: 295000,
    essentialsSpent: 100000,
    desiresSpent: 60000,
    savingsSpent: 30000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
  },
  {
    year: 2026,
    month: 4,
    totalSpent: 310000,
    budgetAmount: 300000,
    essentialsSpent: 155000,
    desiresSpent: 93000,
    savingsSpent: 62000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
  },
  {
    year: 2026,
    month: 5,
    totalSpent: 120000,
    budgetAmount: 300000,
    essentialsSpent: 60000,
    desiresSpent: 36000,
    savingsSpent: 24000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
  },
];

describe("TrendsSection", () => {
  it("renders Select with Monthly Spending as default and the spending chart", () => {
    render(
      <TrendsSection
        trendData={mockTrendData}
        trendMonths={6}
        onToggle={() => {}}
        currency="USD"
      />,
    );

    expect(screen.getByLabelText("Select trend chart")).toBeInTheDocument();
    // Monthly Spending appears in both the Select trigger and chart title
    expect(screen.getAllByText("Monthly Spending").length).toBeGreaterThanOrEqual(1);
  });

  it("renders nothing when data is empty", () => {
    const { container } = render(
      <TrendsSection
        trendData={[]}
        trendMonths={6}
        onToggle={() => {}}
        currency="USD"
      />,
    );

    expect(container.innerHTML).toBe("");
  });

  it("renders toggle group with 6M and 12M options", () => {
    render(
      <TrendsSection
        trendData={mockTrendData}
        trendMonths={6}
        onToggle={() => {}}
        currency="USD"
      />,
    );

    expect(screen.getByRole("radio", { name: "6 months" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "12 months" })).toBeInTheDocument();
  });

  it("calls onToggle when 12M is clicked", async () => {
    const user = userEvent.setup();
    let toggledTo: number | null = null;
    const onToggle = (months: 6 | 12) => {
      toggledTo = months;
    };

    render(
      <TrendsSection
        trendData={mockTrendData}
        trendMonths={6}
        onToggle={onToggle}
        currency="USD"
      />,
    );

    const toggle12M = screen.getByRole("radio", { name: "12 months" });
    await user.click(toggle12M);

    expect(toggledTo).toBe(12);
  });

  it("marks current toggle value as active", () => {
    render(
      <TrendsSection
        trendData={mockTrendData}
        trendMonths={12}
        onToggle={() => {}}
        currency="USD"
      />,
    );

    const toggle12M = screen.getByRole("radio", { name: "12 months" });
    expect(toggle12M).toHaveAttribute("data-state", "on");

    const toggle6M = screen.getByRole("radio", { name: "6 months" });
    expect(toggle6M).toHaveAttribute("data-state", "off");
  });

  it("switches to Category Split chart when selected", async () => {
    const user = userEvent.setup();
    render(
      <TrendsSection
        trendData={mockTrendData}
        trendMonths={6}
        onToggle={() => {}}
        currency="USD"
      />,
    );

    // Open Select and choose Category Split
    const trigger = screen.getByLabelText("Select trend chart");
    await user.click(trigger);
    const categorySplitOption = await screen.findByRole("option", { name: "Category Split" });
    await user.click(categorySplitOption);

    // Category Split appears in both Select trigger and chart title
    expect(screen.getAllByText("Category Split").length).toBeGreaterThanOrEqual(2);
  });
});
