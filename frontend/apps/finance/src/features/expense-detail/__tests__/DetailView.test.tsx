import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Expense, Tag } from "@gofin/core";
import { DetailView } from "../components/DetailView";

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

function renderDetail(expense: Expense, history: Expense[] = [expense]) {
  return render(
    <DetailView
      expense={expense}
      currency="USD"
      tags={tags}
      history={history}
      proRataGroup={[]}
      currentYear={2026}
      currentMonth={5}
      onCorrectClick={vi.fn()}
      onDeleteClick={vi.fn()}
      deleting={false}
      deleteError={null}
    />,
  );
}

describe("DetailView money display", () => {
  it("labels same-currency amounts as the period amount without duplicate rows", () => {
    renderDetail(
      buildExpense({
        transactionCurrencyCode: "USD",
        originalTransactionAmountInMinorUnits: 5000,
        reportingCurrencyCode: "USD",
        reportingAmountInMinorUnits: 5000,
        sourceToTargetExchangeRate: "1",
        exchangeRateSource: "identity",
      }),
    );

    expect(screen.getByText("Period Amount")).toBeInTheDocument();
    expect(screen.getByText("$50.00")).toBeInTheDocument();
    expect(screen.queryByText("Transaction Amount")).not.toBeInTheDocument();
    expect(screen.queryByText("Budget Impact")).not.toBeInTheDocument();
  });

  it("shows transaction, budget impact, rate, and timestamp for foreign currency", () => {
    renderDetail(
      buildExpense({
        transactionCurrencyCode: "EUR",
        originalTransactionAmountInMinorUnits: 1250,
        reportingCurrencyCode: "USD",
        reportingAmountInMinorUnits: 1364,
        sourceToTargetExchangeRate: "1.0912",
        exchangeRateSource: "open_exchange_rates",
        exchangeRateTimestamp: "2026-08-14T10:00:00Z",
      }),
    );

    expect(screen.getByText("Transaction Amount")).toBeInTheDocument();
    expect(screen.getByText("€12.50")).toBeInTheDocument();
    expect(screen.getByText("Budget Impact")).toBeInTheDocument();
    expect(screen.getByText("$13.64")).toBeInTheDocument();
    expect(screen.getByText("Exchange Rate")).toBeInTheDocument();
    expect(screen.getByText("1.0912")).toBeInTheDocument();
    expect(screen.getByText("Rate Timestamp")).toBeInTheDocument();
    expect(screen.getByText("2026-08-14T10:00:00Z")).toBeInTheDocument();
  });
});

describe("DetailView delete button", () => {
  it("shows a delete button for active, current-period expenses", () => {
    renderDetail(buildExpense({ status: "active", periodYear: 2026, periodMonth: 5 }));
    expect(screen.getByRole("button", { name: /delete/i })).toBeInTheDocument();
  });

  it("hides the delete button for corrected expenses", () => {
    renderDetail(buildExpense({ status: "corrected", periodYear: 2026, periodMonth: 5 }));
    expect(screen.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
  });

  it("hides the delete button for past-period expenses", () => {
    renderDetail(buildExpense({ status: "active", periodYear: 2026, periodMonth: 4 }));
    expect(screen.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
  });

  it("opens a confirmation dialog when delete is clicked", async () => {
    const user = userEvent.setup();
    renderDetail(buildExpense());

    await user.click(screen.getByRole("button", { name: /delete/i }));

    expect(
      screen.getByText(/are you sure you want to delete this expense/i),
    ).toBeInTheDocument();
  });
});
