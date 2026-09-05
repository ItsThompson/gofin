import { describe, it, expect } from "vitest";
import type { Expense } from "@gofin/core";
import { hasSameCurrencySnapshot } from "../utils/moneyFacts";

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

describe("moneyFacts", () => {
  it("detects same-currency snapshots", () => {
    const expense = buildExpense({
      transactionCurrencyCode: "USD",
      reportingCurrencyCode: "USD",
    });

    expect(hasSameCurrencySnapshot(expense)).toBe(true);
  });

  it("detects foreign-currency snapshots", () => {
    const expense = buildExpense({
      transactionCurrencyCode: "EUR",
      reportingCurrencyCode: "USD",
    });

    expect(hasSameCurrencySnapshot(expense)).toBe(false);
  });
});
