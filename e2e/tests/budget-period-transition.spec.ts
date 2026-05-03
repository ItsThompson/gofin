import { test, expect } from "@playwright/test";
import { registerAndOnboard } from "../helpers/auth.js";
import {
  apiCreatePeriod,
  apiCreateExpense,
  apiGetTags,
} from "../helpers/api.js";

test.describe("Budget Period Transition", () => {
  test("shows new-month prompt when transitioning from a past period to the current month", async ({
    page,
  }) => {
    // Compute a past month (2 months ago) so it never collides with the current month.
    const now = new Date();
    const currentYear = now.getFullYear();
    const currentMonth = now.getMonth() + 1;
    const pastDate = new Date(currentYear, currentMonth - 3, 15);
    const pastYear = pastDate.getFullYear();
    const pastMonth = pastDate.getMonth() + 1;
    const pastDateISO = `${pastYear}-${String(pastMonth).padStart(2, "0")}-15`;

    // Step 1: Register and onboard (no period created yet for current month)
    await registerAndOnboard(page, { budget: "2000" });

    // After onboarding, the dashboard shows the "Set Up [Month]" prompt
    // for the current month. We need to NOT confirm it: instead, create a
    // past-month period via API so the user has historical data.

    // Step 2: Create a past-month period via direct API calls
    await apiCreatePeriod(page.request, {
      year: pastYear,
      month: pastMonth,
      budgetAmount: 200_000, // $2,000 in cents
    });

    // Step 3: Fetch tags (created during onboarding) and log expenses into the past period
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
      name: "Past Groceries",
      amount: 15_000, // $150
      expenseType: "essentials",
      tagId: firstTag.id,
      expenseDate: pastDateISO,
      periodYear: pastYear,
      periodMonth: pastMonth,
    });

    // Step 4: Navigate to the dashboard, which loads the CURRENT month.
    // Since no period exists for the current month, the "Set Up" prompt appears.
    await page.goto("/dashboard");

    // Verify the new-month prompt is visible
    const monthName = now.toLocaleString("en-US", { month: "long" });
    await expect(
      page.getByRole("heading", { name: new RegExp(`Set Up ${monthName}`) }),
    ).toBeVisible();
    await expect(
      page.getByText("No budget period exists for this month"),
    ).toBeVisible();

    // Step 5: Confirm the defaults to create the current month's period
    await page
      .getByRole("button", { name: new RegExp(`Create ${monthName} Period`) })
      .click();

    // Step 6: Verify the dashboard is fresh with $0 spent
    await expect(page.getByText("Total Budget")).toBeVisible();
    await expect(page.getByText("Total Spent")).toBeVisible();

    // The fresh period should show $0.00 for Total Spent
    const totalSpentCard = page.locator("text=Total Spent").locator("..");
    await expect(totalSpentCard).toContainText("$0.00");

    // No recent expenses in the current month
    await expect(page.getByText("No expenses yet")).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Log your first expense" }),
    ).toBeVisible();
  });
});
