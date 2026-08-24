import { describe, it, expect } from "vitest";
import { computeChanges } from "../utils/computeChanges";
import type { Expense, Tag } from "@gofin/core";

const baseTags: Tag[] = [
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-transport", name: "Transport", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
];

const baseExpense: Expense = {
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
};

describe("computeChanges", () => {
  it("returns empty array when nothing changed", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 5000,
      expenseType: "essentials" as const,
      tagId: "tag-food",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([]);
  });

  it("detects name change", () => {
    const corrected = {
      name: "Weekly Groceries",
      transactionAmount: 5000,
      expenseType: "essentials" as const,
      tagId: "tag-food",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([
      { field: "Name", from: "Groceries", to: "Weekly Groceries" },
    ]);
  });

  it("detects amount change with formatted currency diff", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 7500,
      expenseType: "essentials" as const,
      tagId: "tag-food",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([
      { field: "Amount", from: "$50.00", to: "$75.00" },
    ]);
  });

  it("detects expense type change", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 5000,
      expenseType: "desires" as const,
      tagId: "tag-food",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([
      { field: "Type", from: "essentials", to: "desires" },
    ]);
  });

  it("detects tag change and resolves tag names", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 5000,
      expenseType: "essentials" as const,
      tagId: "tag-transport",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([
      { field: "Tag", from: "Food", to: "Transport" },
    ]);
  });

  it("falls back to tag ID when tag name is not found", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 5000,
      expenseType: "essentials" as const,
      tagId: "tag-unknown",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([
      { field: "Tag", from: "Food", to: "tag-unknown" },
    ]);
  });

  it("detects date change", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 5000,
      expenseType: "essentials" as const,
      tagId: "tag-food",
      expenseDate: "2026-05-10",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toEqual([
      { field: "Date", from: "2026-05-02", to: "2026-05-10" },
    ]);
  });

  it("detects multiple changes at once", () => {
    const corrected = {
      name: "Updated Groceries",
      transactionAmount: 7500,
      expenseType: "desires" as const,
      tagId: "tag-transport",
      expenseDate: "2026-05-10",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "USD");
    expect(result).toHaveLength(5);
    expect(result[0].field).toBe("Name");
    expect(result[1].field).toBe("Amount");
    expect(result[2].field).toBe("Type");
    expect(result[3].field).toBe("Tag");
    expect(result[4].field).toBe("Date");
  });

  it("works with different currency formatting", () => {
    const corrected = {
      name: "Groceries",
      transactionAmount: 7500,
      expenseType: "essentials" as const,
      tagId: "tag-food",
      expenseDate: "2026-05-02",
    };

    const result = computeChanges(baseExpense, corrected, baseTags, "EUR");
    expect(result).toHaveLength(1);
    expect(result[0].field).toBe("Amount");
    // EUR formatting should differ from USD
    expect(result[0].from).not.toBe(result[0].to);
  });

  it("accepts a full Expense object as corrected values", () => {
    const correctedExpense: Expense = {
      ...baseExpense,
      id: "exp-2",
      name: "Updated Groceries",
      transactionAmount: 6000,
      correctsId: "exp-1",
    };

    const result = computeChanges(baseExpense, correctedExpense, baseTags, "USD");
    expect(result).toEqual([
      { field: "Name", from: "Groceries", to: "Updated Groceries" },
      { field: "Amount", from: "$50.00", to: "$60.00" },
    ]);
  });
});
