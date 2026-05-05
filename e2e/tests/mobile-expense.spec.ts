import { test, expect } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
  logExpense,
} from "../helpers/auth.js";

test.describe("Mobile Expense Logging", () => {
  test.use({
    viewport: { width: 375, height: 812 },
  });

  test("logs an expense on a mobile viewport and verifies it appears in the list", async ({
    page,
  }) => {
    // Setup: register, onboard, create period on mobile
    await registerAndOnboard(page, { budget: "1000" });
    await confirmNewPeriod(page);

    // Step 1: Verify mobile layout
    // Desktop nav should be hidden, mobile hamburger should be visible
    await expect(
      page.getByRole("button", { name: "Open menu" }),
    ).toBeVisible();

    // The mobile FAB (floating action button) for "Log Expense" should be visible
    const fab = page.getByRole("button", { name: "Log Expense" });
    await expect(fab).toBeVisible();

    // Step 2: Log an expense using the mobile FAB
    await fab.click();
    await expect(page).toHaveURL(/\/expenses\/new/);

    // Fill the expense form
    await page.getByLabel("Name").fill("Mobile Coffee");
    await page.getByLabel("Amount").fill("5.50");
    await page.getByLabel("Essentials").check();
    await page.getByRole("button", { name: "Log Expense" }).click();
    await page.waitForURL("**/dashboard");

    // Step 3: Verify the expense appears on the dashboard
    await expect(page.getByText("Mobile Coffee")).toBeVisible();
    await expect(page.getByText("$5.50").first()).toBeVisible();

    // Step 4: Navigate to expense log via mobile menu
    await page.getByRole("button", { name: "Open menu" }).click();
    await page.getByRole("link", { name: "Expenses" }).click();
    await expect(page).toHaveURL(/\/expenses/);

    // Step 5: Verify the expense is in the mobile list view
    // On mobile, expenses render as a list (not a table)
    await expect(page.locator("text=Mobile Coffee").locator("visible=true").first()).toBeVisible();
    await expect(page.locator("text=$5.50").locator("visible=true").first()).toBeVisible();
  });
});
