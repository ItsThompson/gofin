import { describe, expect, it } from "vitest";
import type { Tag } from "@gofin/core";
import { createExpenseSuggestionPatch } from "../suggestionAutofill";
import type { ExpenseSuggestion } from "../types";

const tags: Tag[] = [
  {
    id: "tag-food",
    name: "Food",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

const suggestion: ExpenseSuggestion = {
  name: "Groceries",
  transactionAmount: 1299,
  transactionCurrency: "USD",
  amount: 1299,
  currency: "USD",
  expenseType: "essentials",
  tagId: "tag-food",
  frequency: 7,
  lastUsedAt: "2026-05-28T19:02:11Z",
  recencyBucket: "last_7_days",
  frecencyScore: 42,
};

describe("createExpenseSuggestionPatch", () => {
  it("formats suggestion fields for expense forms", () => {
    expect(createExpenseSuggestionPatch(suggestion, tags)).toEqual({
      name: "Groceries",
      amountDollars: "12.99",
      currency: "USD",
      expenseType: "essentials",
      tagId: "tag-food",
    });
  });

  it("returns a null tagId when the suggestion tag is stale", () => {
    expect(
      createExpenseSuggestionPatch(
        { ...suggestion, tagId: "deleted-tag" },
        tags,
      ),
    ).toEqual({
      name: "Groceries",
      amountDollars: "12.99",
      currency: "USD",
      expenseType: "essentials",
      tagId: null,
    });
  });

  it("formats zero-decimal suggestion amounts with their currency precision", () => {
    expect(
      createExpenseSuggestionPatch({ ...suggestion, transactionAmount: 1299, transactionCurrency: "JPY", amount: 1299, currency: "JPY" }, tags),
    ).toEqual({
      name: "Groceries",
      amountDollars: "1299",
      currency: "JPY",
      expenseType: "essentials",
      tagId: "tag-food",
    });
  });

  it("uses transaction fields when deprecated fields differ", () => {
    // Simulates a response where the new transaction fields are canonical.
    // The patch should use the transaction currency and amount, not the deprecated ones.
    expect(
      createExpenseSuggestionPatch(
        { ...suggestion, transactionAmount: 15000, transactionCurrency: "EUR", amount: 1299, currency: "USD" },
        tags,
      ),
    ).toEqual({
      name: "Groceries",
      amountDollars: "150.00",
      currency: "EUR",
      expenseType: "essentials",
      tagId: "tag-food",
    });
  });
});
