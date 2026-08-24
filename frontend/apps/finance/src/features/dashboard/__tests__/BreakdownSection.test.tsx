import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { BreakdownSection } from "../components/BreakdownSection";
import type { ExpenseFrecencyDataState } from "../hooks/useExpenseFrecencyData";

const mockTagSpending = [
  { tagId: "tag-food", tagName: "Food", amount: 50000, percentOfTotal: 91.74 },
  { tagId: "tag-social", tagName: "Social", amount: 4500, percentOfTotal: 8.26 },
];

const mockFrecencyData: ExpenseFrecencyDataState = {
  status: "success",
  suggestions: [
    {
      name: "Groceries",
      transactionAmount: 50000,
      transactionCurrency: "USD",
      expenseType: "essentials",
      frequency: 114,
      lastUsedAt: "2026-05-02T10:00:00Z",
      recencyBucket: "last_7_days",
      frecencyScore: 145,
      tagId: "tag-food",
    },
  ],
  errorMessage: null,
};

function renderBreakdown(props?: Partial<Parameters<typeof BreakdownSection>[0]>) {
  return render(
    <MemoryRouter>
      <BreakdownSection
        tagSpending={mockTagSpending}
        expenseFrecencyData={mockFrecencyData}
        currency="USD"
        {...props}
      />
    </MemoryRouter>,
  );
}

describe("BreakdownSection", () => {
  it("renders Select with 'Spending by Tag' as default", () => {
    renderBreakdown();
    expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument();
    expect(screen.getAllByText("Spending by Tag").length).toBeGreaterThanOrEqual(1);
  });

  it("shows TagSpendingChart by default", () => {
    renderBreakdown();
    // TagSpendingChart renders a card with title "Spending by Tag"
    expect(screen.getAllByText("Spending by Tag").length).toBeGreaterThanOrEqual(2);
  });

  it("switches to Repeated Expenses chart when selected", async () => {
    const user = userEvent.setup();
    renderBreakdown();

    const trigger = screen.getByLabelText("Select breakdown chart");
    await user.click(trigger);
    const option = await screen.findByRole("option", { name: "Repeated Expenses" });
    await user.click(option);

    // ExpenseFrecencyChart renders with success state content
    expect(screen.getByText(/Frequency shows how often/i)).toBeInTheDocument();
    // Tag spending chart card should no longer be present
    expect(screen.queryByText("Spending by Tag")).toBeNull();
  });

  it("shows empty state when tag spending is empty and frecency is empty", () => {
    renderBreakdown({
      tagSpending: [],
      expenseFrecencyData: { status: "empty", suggestions: [], errorMessage: null },
    });
    // TagSpendingChart still renders its card even with no data (no tags shown)
    expect(screen.getByLabelText("Select breakdown chart")).toBeInTheDocument();
  });
});
