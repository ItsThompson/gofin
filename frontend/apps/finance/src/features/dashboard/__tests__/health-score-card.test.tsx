import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { HealthScoreCard } from "../components/widgets/HealthScoreCard";
import type { HealthScore } from "../../../types";

// ResponsiveContainer won't render children without real DOM dimensions.
// Mock it so the ring chart internals execute in jsdom.
vi.mock("recharts", async (importOriginal) => {
  const actual = await importOriginal<typeof import("recharts")>();
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 160, height: 160 }}>{children}</div>
    ),
  };
});

function buildHealthScore(overrides?: Partial<HealthScore>): HealthScore {
  return {
    year: 2026,
    month: 5,
    total: 56,
    band: "amber",
    provisional: false,
    formulaVersion: 1,
    components: [
      { key: "savings_achievement", score: 20, max: 30, detail: "Saved $400 of $600 target" },
      { key: "budget_adherence", score: 30, max: 30, detail: "Spent $2,000 of $2,400 plan" },
      { key: "allocation_balance", score: 6, max: 40, detail: "Desires 12 pts over target share" },
    ],
    insight: {
      summary: "Your category balance is the softest score this month.",
      driver: "allocation_balance",
      nudge:
        "Desires is running 12 pts over its target share. Shifting spend toward Savings could recover up to 34 points.",
    },
    ...overrides,
  };
}

function renderCard(score: HealthScore | null) {
  return render(
    <MemoryRouter>
      <HealthScoreCard score={score} />
    </MemoryRouter>,
  );
}

describe("HealthScoreCard", () => {
  it("renders the ring with the total and the band label colored by band", () => {
    const { container } = renderCard(buildHealthScore());
    expect(screen.getByText("56")).toBeInTheDocument();
    // amber band -> "Drifting" label and the amber color token on the ring.
    expect(screen.getByText("Drifting")).toBeInTheDocument();
    expect(container.querySelector(".text-amber-500")).not.toBeNull();
  });

  it("renders one bar per component with score/max and the detail line", () => {
    renderCard(buildHealthScore());

    expect(screen.getByText("Savings")).toBeInTheDocument();
    expect(screen.getByText("Budget adherence")).toBeInTheDocument();
    expect(screen.getByText("Allocation balance")).toBeInTheDocument();

    expect(screen.getByText("20/30")).toBeInTheDocument();
    expect(screen.getByText("30/30")).toBeInTheDocument();
    expect(screen.getByText("6/40")).toBeInTheDocument();

    expect(screen.getByText("Saved $400 of $600 target")).toBeInTheDocument();
    expect(screen.getByText("Desires 12 pts over target share")).toBeInTheDocument();

    expect(screen.getAllByRole("progressbar")).toHaveLength(3);
  });

  it("highlights the driver bar", () => {
    renderCard(buildHealthScore());
    const driverRow = screen.getByTestId("sub-score-allocation_balance");
    const savingsRow = screen.getByTestId("sub-score-savings_achievement");
    expect(driverRow.className).toContain("ring-1");
    expect(savingsRow.className).not.toContain("ring-1");
  });

  it("renders the insight summary and nudge with a lucide icon (no emoji)", () => {
    const { container } = renderCard(buildHealthScore());
    expect(
      screen.getByText("Your category balance is the softest score this month."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Shifting spend toward Savings could recover up to 34 points\./),
    ).toBeInTheDocument();
    // lucide renders an inline svg; no emoji fallback.
    expect(container.querySelector("svg")).not.toBeNull();
    expect(screen.queryByText(/💡/)).toBeNull();
  });

  it("shows the 'Month to date' badge when provisional", () => {
    renderCard(buildHealthScore({ provisional: true }));
    expect(screen.getByText("Month to date")).toBeInTheDocument();
  });

  it("hides the provisional badge for a closed month", () => {
    renderCard(buildHealthScore({ provisional: false }));
    expect(screen.queryByText("Month to date")).toBeNull();
  });

  it("shows the configure-budget prompt with a Budget Settings link", () => {
    renderCard(buildHealthScore({ configureBudget: true }));
    expect(
      screen.getByText("Set a budget to see your health score."),
    ).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /budget settings/i });
    expect(link).toHaveAttribute("href", "/settings");
    // No score UI in the configure state.
    expect(screen.queryByRole("progressbar")).toBeNull();
  });

  it("shows two bars and the savings note when savings is dropped", () => {
    const dropped = buildHealthScore({
      total: 88,
      band: "green",
      components: [
        { key: "budget_adherence", score: 43, max: 43, detail: "Spent $3,000 of $3,000 plan" },
        { key: "allocation_balance", score: 45, max: 57, detail: "Essentials 5 pts over target share" },
      ],
      insight: {
        summary: "Your category balance is the softest score this month.",
        driver: "allocation_balance",
        nudge: "Essentials is running 5 pts over its target share. Shifting spend toward Desires could recover up to 12 points.",
      },
    });
    renderCard(dropped);

    expect(screen.getAllByRole("progressbar")).toHaveLength(2);
    expect(screen.getByText("Savings isn't budgeted this month.")).toBeInTheDocument();
    expect(screen.queryByText("Savings")).toBeNull();
  });

  it("renders nothing when the score is null", () => {
    const { container } = renderCard(null);
    expect(container).toBeEmptyDOMElement();
  });
});
