import { describe, expect, it } from "vitest";
import type {
  CorrectExpenseRequest,
  CreateExpenseRequest,
  ExpenseResponse,
} from "@gofin/core";

import { mockExpenses, mockUpcomingProRata } from "../data";
import { expensesHandlers } from "../handlers/expenses";
import { resolveMockRequest } from "./drive";

describe("mock expense amounts", () => {
  const original = mockExpenses[0];

  it("reads the current amount field when an expense is created", async () => {
    const request: CreateExpenseRequest = {
      name: "Groceries",
      amountInTransactionCurrencyMinorUnits: 12345,
      transactionCurrencyCode: "USD",
      expenseType: "essentials",
      tagId: original.tagId,
      expenseDateIso: original.expenseDateIso,
      periodYear: original.periodYear,
      periodMonth: original.periodMonth,
      clientGeneratedIdempotencyKey: crypto.randomUUID(),
    };
    const response = await resolveMockRequest(expensesHandlers, "/api/expenses", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    const { expense } = (await response.json()) as ExpenseResponse;

    expect(response.status).toBe(201);
    expect(expense.originalTransactionAmountInMinorUnits).toBe(12345);
    expect(expense.reportingAmountInMinorUnits).toBe(12345);
    expect(expense.reportingCurrencyCode).toBe("USD");
    expect(expense).not.toHaveProperty("amount");
  });

  it("reads the current amount field when an expense is corrected", async () => {
    const request: CorrectExpenseRequest = {
      name: original.name,
      amountInTransactionCurrencyMinorUnits: 6789,
      expenseType: original.expenseType,
      tagId: original.tagId,
      expenseDateIso: original.expenseDateIso,
    };
    const response = await resolveMockRequest(
      expensesHandlers,
      `/api/expenses/${original.id}/correct`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      },
    );
    const { expense } = (await response.json()) as ExpenseResponse;

    expect(response.status).toBe(201);
    expect(expense.originalTransactionAmountInMinorUnits).toBe(6789);
    expect(expense.reportingAmountInMinorUnits).toBe(6789);
    expect(expense.correctsId).toBe(original.id);
    expect(expense).not.toHaveProperty("amount");
  });

  it("provides the installment amount in upcoming schedules", () => {
    expect(mockUpcomingProRata).toHaveLength(1);
    expect(mockUpcomingProRata[0].installmentAmountInMinorUnits).toBe(50000);
    expect(mockUpcomingProRata[0]).not.toHaveProperty("amount");
  });
});
