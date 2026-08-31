import { describe, it, expect } from "vitest";
import type { Expense } from "@gofin/core";
import { hasSameCurrencySnapshot } from "../utils/moneyFacts";

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

describe("moneyFacts", () => {
  it("detects same-currency snapshots", () => {
    const expense = buildExpense({
      transactionCurrency: "USD",
      reportingCurrency: "USD",
    });

    expect(hasSameCurrencySnapshot(expense)).toBe(true);
  });

  it("detects foreign-currency snapshots", () => {
    const expense = buildExpense({
      transactionCurrency: "EUR",
      reportingCurrency: "USD",
    });

    expect(hasSameCurrencySnapshot(expense)).toBe(false);
  });
});
