import { test, expect } from "@playwright/test";
import {
  registerUser,
  completeOnboarding,
  confirmNewPeriod,
  logExpense,
} from "../helpers/auth.js";

test.describe("Registration → Onboarding → First Expense", () => {
  test("registers, completes onboarding, creates period, logs expense, and verifies dashboard", async ({
    page,
  }) => {
    // Step 1: Register a new user
    const user = await registerUser(page);
    await expect(page).toHaveURL(/\/onboarding/);

    // Step 2: Complete all onboarding steps with a budget
    await completeOnboarding(page, { budget: "2000" });
    await expect(page).toHaveURL(/\/dashboard/);

    // Step 3: Confirm the new month period on the dashboard
    await confirmNewPeriod(page);

    // Verify the dashboard shows the budget
    await expect(page.getByText("Total Budget")).toBeVisible();
    await expect(page.getByText("$2,000.00").first()).toBeVisible();
    await expect(page.getByText("$0.00").first()).toBeVisible();

    // Step 4: Log an expense
    await logExpense(page, {
      name: "Grocery Shopping",
      amount: "42.50",
      type: "essentials",
    });

    // Step 5: Verify the dashboard reflects the new expense
    // Recent expenses section should show the expense
    const recentExpenses = page
      .getByText("Recent Expenses")
      .locator('xpath=ancestor::*[@data-slot="card"][1]');
    await expect(recentExpenses.getByText("Grocery Shopping")).toBeVisible();
    await expect(recentExpenses.getByText("$42.50")).toBeVisible();

    // Total Spent should update
    await expect(page.getByText("Total Spent")).toBeVisible();
  });
});
