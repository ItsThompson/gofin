import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Expense, Tag } from "@gofin/core";
import { CorrectionTimeline } from "../components/CorrectionTimeline";

const tags: Tag[] = [
  {
    id: "tag-food",
    name: "Food",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

function buildExpense(overrides: Partial<Expense> = {}): Expense {
  return {
    id: "exp-1",
    userId: "user-1",
    name: "Groceries",
    transactionCurrencyCode: "USD",
    originalTransactionAmountInMinorUnits: 5000,
    reportingAmountInMinorUnits: 5000,
    reportingCurrencyCode: "USD",
    expenseType: "essentials",
    tagId: "tag-food",
    expenseDateIso: "2026-05-02",
    periodYear: 2026,
    periodMonth: 5,
    status: "active",
    isProRata: false,
    createdAt: "2026-05-02T10:00:00Z",
    ...overrides,
  };
}

describe("CorrectionTimeline", () => {
  it("renders each row with its own transaction and reporting snapshot and status", () => {
    const original = buildExpense({
      id: "exp-original",
      name: "Original",
      status: "corrected",
      transactionCurrencyCode: "EUR",
      originalTransactionAmountInMinorUnits: 1250,
      reportingCurrencyCode: "USD",
      reportingAmountInMinorUnits: 1364,
      sourceToTargetExchangeRate: "1.0912",
    });
    const correction = buildExpense({
      id: "exp-correction",
      name: "Updated Coffee",
      status: "active",
      correctsId: "exp-original",
      transactionCurrencyCode: "USD",
      originalTransactionAmountInMinorUnits: 1400,
      reportingCurrencyCode: "USD",
      reportingAmountInMinorUnits: 1400,
      sourceToTargetExchangeRate: "1",
    });

    render(
      <CorrectionTimeline
        entries={[original, correction]}
        currency="USD"
        tags={tags}
        currentExpenseId="exp-correction"
      />,
    );

    expect(screen.getByText("Original")).toBeInTheDocument();
    expect(screen.getByText("Correction 1")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getAllByText("Corrected").length).toBeGreaterThanOrEqual(1);

    // The foreign-currency original shows its transaction amount and its
    // reporting amount in parentheses.
    // €12.50 appears in the row and in the Amount change chip.
    expect(screen.getAllByText(/€12\.50/).length).toBeGreaterThan(0);
    expect(screen.getByText(/\(\$13\.64\)/)).toBeInTheDocument();
    // The same-currency correction shows the period amount without parentheses.
    expect(screen.getByText(/Updated Coffee · \$14\.00/)).toBeInTheDocument();
  });

  it("does not duplicate reporting amount for same-currency rows", () => {
    const entry = buildExpense({
      id: "exp-original",
      transactionCurrencyCode: "USD",
      originalTransactionAmountInMinorUnits: 5000,
      reportingCurrencyCode: "USD",
      reportingAmountInMinorUnits: 5000,
      sourceToTargetExchangeRate: "1",
    });

    const { container } = render(
      <CorrectionTimeline
        entries={[entry]}
        currency="USD"
        tags={tags}
        currentExpenseId="exp-original"
      />,
    );

    expect(screen.getByText(/\$50\.00/)).toBeInTheDocument();
    expect(container.textContent).not.toContain("($50.00)");
  });
});
