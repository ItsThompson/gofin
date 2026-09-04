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
  originalTransactionAmountInMinorUnits: 1299,
  transactionCurrencyCode: "USD",
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
      createExpenseSuggestionPatch({ ...suggestion, originalTransactionAmountInMinorUnits: 1299, transactionCurrencyCode: "JPY" }, tags),
    ).toEqual({
      name: "Groceries",
      amountDollars: "1299",
      currency: "JPY",
      expenseType: "essentials",
      tagId: "tag-food",
    });
  });
});
