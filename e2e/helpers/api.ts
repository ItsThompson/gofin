import type { APIRequestContext } from "@playwright/test";

/**
 * Direct API helpers for test setup.
 *
 * These use Playwright's APIRequestContext (which shares the browser's
 * cookie jar) to create backend data without navigating through the UI.
 * Used when tests need pre-existing data in specific states that would
 * be tedious or impossible to set up through the UI alone.
 */

interface CreatePeriodParams {
  year: number;
  month: number;
  budgetAmount: number;
  reportingCurrency?: string;
  essentialsPercent?: number;
  desiresPercent?: number;
  savingsPercent?: number;
}

interface CreateExpenseParams {
  name: string;
  amount: number;
  currency?: string;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  expenseDate: string;
  periodYear: number;
  periodMonth: number;
}

/**
 * Create a budget period via the API.
 * The request context must already be authenticated (cookies set via login/register).
 */
export async function apiCreatePeriod(
  request: APIRequestContext,
  params: CreatePeriodParams,
) {
  const response = await request.post("/api/finance/periods", {
    data: {
      year: params.year,
      month: params.month,
      budgetAmount: params.budgetAmount,
      reportingCurrency: params.reportingCurrency ?? "USD",
      essentialsPercent: params.essentialsPercent ?? 50,
      desiresPercent: params.desiresPercent ?? 30,
      savingsPercent: params.savingsPercent ?? 20,
    },
  });

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(
      `Failed to create period (${response.status()}): ${body}`,
    );
  }

  return response.json();
}

/**
 * Create an expense via the API.
 * The request context must already be authenticated.
 */
export async function apiCreateExpense(
  request: APIRequestContext,
  params: CreateExpenseParams,
) {
  const response = await request.post("/api/expenses", {
    data: {
      name: params.name,
      amount: params.amount,
      currency: params.currency ?? "USD",
      expenseType: params.expenseType,
      tagId: params.tagId,
      expenseDate: params.expenseDate,
      periodYear: params.periodYear,
      periodMonth: params.periodMonth,
    },
  });

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(
      `Failed to create expense (${response.status()}): ${body}`,
    );
  }

  return response.json();
}

/**
 * Fetch the user's tags via the API.
 * Returns the list of tags (created during onboarding as defaults).
 */
export async function apiGetTags(
  request: APIRequestContext,
): Promise<{ tags: Array<{ id: string; name: string }> }> {
  const response = await request.get("/api/finance/tags");

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Failed to fetch tags (${response.status()}): ${body}`);
  }

  return response.json();
}
