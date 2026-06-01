import { test, expect } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
  logExpense,
} from "../helpers/auth.js";
import { apiGetTags } from "../helpers/api.js";

function currentPeriod() {
  const now = new Date();
  return { year: now.getFullYear(), month: now.getMonth() + 1 };
}

test.describe("Expense autocomplete smoke", () => {
  test("routes suggestions through the gateway and submits an edited selected suggestion", async ({ page }) => {
    await registerAndOnboard(page, { budget: "2000" });
    await confirmNewPeriod(page);

    const { year, month } = currentPeriod();
    const { tags } = await apiGetTags(page.request);
    const foodTag = tags[0];

    await logExpense(page, {
      name: "Autocomplete Coffee",
      amount: "12.00",
      type: "desires",
    });

    await page.goto("/expenses/new");

    const expensesResponse = await page.request.get(`/api/expenses?year=${year}&month=${month}`);
    expect(expensesResponse.ok()).toBe(true);

    const suggestionsResponse = await page.request.get("/api/expenses/suggestions?page=1&pageSize=50");
    expect(suggestionsResponse.ok()).toBe(true);
    const suggestionsBody = await suggestionsResponse.json();
    expect(suggestionsBody.data.some((suggestion: { name: string }) => suggestion.name === "Autocomplete Coffee")).toBe(true);

    await page.getByLabel("Name").fill("Auto");
    await page.getByText("Autocomplete Coffee").click();

    await expect(page.getByLabel("Name")).toHaveValue("Autocomplete Coffee");
    await expect(page.getByLabel("Amount")).toHaveValue("12");
    await expect(page.getByLabel("Desires")).toBeChecked();

    await page.getByLabel("Amount").fill("13.25");
    await page.getByRole("button", { name: "Log Expense" }).click();
    await page.waitForURL("**/dashboard");

    await expect(page.getByText("Autocomplete Coffee").first()).toBeVisible();
    await expect(page.getByText("$13.25").first()).toBeVisible();
    expect(foodTag.id).toBeTruthy();
  });

  test("keeps manual submission working when suggestions fail", async ({ page }) => {
    await registerAndOnboard(page, { budget: "1000" });
    await confirmNewPeriod(page);

    await page.route("**/api/expenses/suggestions**", async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ code: "internal_server_error", message: "failed" }),
      });
    });

    await page.goto("/expenses/new");
    await page.getByLabel("Name").fill("Manual Fallback Expense");
    await page.getByLabel("Amount").fill("8.25");
    await page.getByLabel("Essentials").check();
    await page.getByRole("button", { name: "Log Expense" }).click();
    await page.waitForURL("**/dashboard");

    await expect(page.getByText("Manual Fallback Expense").first()).toBeVisible();
    await expect(page.getByText("$8.25").first()).toBeVisible();
  });

  test("suggestions exclude corrected values and count a pro-rata group once", async ({ page }) => {
    await registerAndOnboard(page, { budget: "2000" });
    await confirmNewPeriod(page);

    await logExpense(page, {
      name: "Corrected Autocomplete",
      amount: "25.00",
      type: "desires",
    });

    await page.getByRole("link", { name: "Expenses" }).click();
    await page.getByText("Corrected Autocomplete").first().click();
    await page.getByRole("button", { name: "Correct This Expense" }).click();
    await page.getByLabel("Amount").fill("30.00");
    await page.getByRole("button", { name: "Save Correction" }).click();
    await expect(page.getByRole("heading", { name: "Expense Detail" })).toBeVisible();
    await page.keyboard.press("Escape");

    await logExpense(page, {
      name: "Autocomplete Subscription",
      amount: "300.00",
      type: "essentials",
      proRata: { months: "3" },
    });

    const response = await page.request.get("/api/expenses/suggestions?page=1&pageSize=50");
    expect(response.ok()).toBe(true);
    const body = await response.json();

    const correctedSuggestion = body.data.find((suggestion: { name: string }) => suggestion.name === "Corrected Autocomplete");
    expect(correctedSuggestion).toMatchObject({ amount: 3000, frequency: 1 });

    const proRataSuggestion = body.data.find((suggestion: { name: string }) => suggestion.name === "Autocomplete Subscription");
    expect(proRataSuggestion).toMatchObject({ frequency: 1, expenseType: "essentials" });
  });
});
