import { test, expect } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
  logExpense,
} from "../helpers/auth.js";

test.describe("Expense Correction", () => {
  test("logs an expense, corrects it, and verifies correction badges and totals", async ({
    page,
  }) => {
    // Setup: register, onboard, create period
    await registerAndOnboard(page, { budget: "1000" });
    await confirmNewPeriod(page);

    // Step 1: Log an expense
    await logExpense(page, {
      name: "Coffee Beans",
      amount: "25.00",
      type: "desires",
    });

    // Step 2: Navigate to expense log
    await page.getByRole("link", { name: "Expenses" }).click();
    await expect(page).toHaveURL(/\/expenses/);

    // Step 3: Click the expense to open the detail modal
    await page.getByText("Coffee Beans").first().click();
    await expect(
      page.getByRole("heading", { name: "Expense Detail" }),
    ).toBeVisible();

    // Verify the expense shows as "Active"
    await expect(page.getByText("Active")).toBeVisible();

    // Step 4: Click "Correct This Expense"
    await page.getByRole("button", { name: "Correct This Expense" }).click();
    await expect(
      page.getByRole("heading", { name: "Correct Expense" }),
    ).toBeVisible();

    // Step 5: Change the amount
    const amountInput = page.getByLabel("Amount");
    await amountInput.clear();
    await amountInput.fill("30.00");

    // Submit the correction
    await page.getByRole("button", { name: "Save Correction" }).click();

    // Step 6: Verify the detail view shows correction history
    // The modal should return to the detail view after correction
    await expect(
      page.getByRole("heading", { name: "Expense Detail" }),
    ).toBeVisible();

    // The correction timeline should show "Original" and "Correction 1"
    await expect(page.getByText("Original")).toBeVisible();
    await expect(page.getByText("Correction 1")).toBeVisible();

    // Original should show "Corrected" badge, correction should show "Active"
    const correctedBadges = page.getByText("Corrected");
    const activeBadges = page.getByText("Active");
    await expect(correctedBadges.first()).toBeVisible();
    await expect(activeBadges.first()).toBeVisible();

    // Close the modal
    await page.keyboard.press("Escape");

    // Step 7: Navigate to dashboard and verify totals reflect the correction
    await page.getByRole("link", { name: "Dashboard" }).click();
    await expect(page).toHaveURL(/\/dashboard/);

    // The corrected amount ($30.00) should appear, not the original ($25.00)
    await expect(page.getByText("$30.00")).toBeVisible();
  });
});
