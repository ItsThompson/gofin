import { test, expect } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
} from "../helpers/auth.js";
import {
  apiCreatePeriod,
  apiCreateExpense,
  apiGetTags,
} from "../helpers/api.js";

test.describe("History Route", () => {
  test("navigates to history via nav link and displays past period with spent/surplus data", async ({
    page,
  }) => {
    // Compute a past month (2 months ago) to avoid collision with current month
    const now = new Date();
    const pastDate = new Date(now.getFullYear(), now.getMonth() - 2, 15);
    const pastYear = pastDate.getFullYear();
    const pastMonth = pastDate.getMonth() + 1;
    const pastDateISO = `${pastYear}-${String(pastMonth).padStart(2, "0")}-15`;
    const pastMonthName = pastDate.toLocaleString("en-US", {
      month: "long",
      year: "numeric",
    });

    // Step 1: Register, onboard, confirm current period
    await registerAndOnboard(page, { budget: "2000" });
    await confirmNewPeriod(page);

    // Step 2: Create a past period with expenses via API
    await apiCreatePeriod(page.request, {
      year: pastYear,
      month: pastMonth,
      budgetAmount: 150_000, // $1,500
    });

    const { tags } = await apiGetTags(page.request);
    const firstTag = tags[0];

    await apiCreateExpense(page.request, {
      name: "Past Rent",
      amount: 80_000, // $800
      expenseType: "essentials",
      tagId: firstTag.id,
      expenseDate: pastDateISO,
      periodYear: pastYear,
      periodMonth: pastMonth,
    });

    await apiCreateExpense(page.request, {
      name: "Past Dining",
      amount: 20_000, // $200
      expenseType: "desires",
      tagId: firstTag.id,
      expenseDate: pastDateISO,
      periodYear: pastYear,
      periodMonth: pastMonth,
    });

    // Step 3: Navigate to History via the navbar link
    await page.getByRole("link", { name: "History" }).click();
    await expect(page).toHaveURL(/\/history/);

    // Step 4: Verify the History page renders with heading
    await expect(page.getByText("Budget History")).toBeVisible();

    // Step 5: Verify the past period card appears with correct data
    const pastPeriodCard = page.getByTestId(
      `period-row-${pastYear}-${pastMonth}`,
    );
    await expect(pastPeriodCard).toBeVisible();
    await expect(pastPeriodCard).toContainText(pastMonthName);
    await expect(pastPeriodCard).toContainText("$1,000.00"); // spent: $800 + $200
    await expect(pastPeriodCard).toContainText("Surplus"); // $1,500 - $1,000 = $500 surplus

    // Step 6: Click into the past period to view read-only dashboard
    await pastPeriodCard.click();
    await expect(page.getByText("Back to History")).toBeVisible();
    await expect(page.getByText("Total Budget")).toBeVisible();
    await expect(page.getByText("Total Spent")).toBeVisible();

    // Step 7: Navigate back to history list
    await page.getByRole("button", { name: /Back to History/ }).click();
    await expect(page.getByText("Budget History")).toBeVisible();
  });

  test("shows empty state when no budget periods exist", async ({ page }) => {
    // Register and onboard but do NOT confirm a period
    await registerAndOnboard(page, { budget: "1000" });

    // Navigate directly to /history (skip the period creation prompt)
    await page.goto("/history");

    // Verify empty state
    await expect(page.getByText("Budget History")).toBeVisible();
    await expect(page.getByText("No budget periods yet.")).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Go to Dashboard" }),
    ).toBeVisible();
  });

  test("history link shows active state in navbar when on /history", async ({
    page,
  }) => {
    await registerAndOnboard(page, { budget: "1000" });
    await confirmNewPeriod(page);

    // Navigate to /history
    await page.getByRole("link", { name: "History" }).click();
    await expect(page).toHaveURL(/\/history/);

    // The active nav link should have the active styling class
    const historyNavLink = page
      .locator("header nav")
      .getByRole("link", { name: "History" });
    await expect(historyNavLink).toHaveClass(/bg-muted/);
  });
});
