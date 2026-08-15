import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
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
    transactionCurrency: "USD",
    transactionAmount: 5000,
    reportingAmount: 5000,
    reportingCurrency: "USD",
    expenseType: "essentials",
    tagId: "tag-food",
    expenseDate: "2026-05-02",
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
    />,
  );
}

describe("DetailView money display", () => {
  it("labels same-currency amounts as the period amount without duplicate rows", () => {
    renderDetail(
      buildExpense({
        transactionCurrency: "USD",
        transactionAmount: 5000,
        reportingCurrency: "USD",
        reportingAmount: 5000,
        exchangeRate: "1",
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
        transactionCurrency: "EUR",
        transactionAmount: 1250,
        reportingCurrency: "USD",
        reportingAmount: 1364,
        exchangeRate: "1.0912",
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
