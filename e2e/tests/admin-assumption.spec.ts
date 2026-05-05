import { test, expect } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
  logExpense,
  loginUser,
} from "../helpers/auth.js";
import { getAdminCredentials } from "../helpers/test-user.js";

test.describe("Admin Identity Assumption", () => {
  test("admin assumes a regular user identity and returns to admin", async ({
    page,
    context,
  }) => {
    // Step 1: Create a regular user with some data
    const regularUser = await registerAndOnboard(page, { budget: "500" });
    await confirmNewPeriod(page);
    await logExpense(page, {
      name: "User Expense",
      amount: "75.00",
      type: "essentials",
    });

    // Verify the expense is visible on the regular user's dashboard
    await expect(page.getByText("User Expense")).toBeVisible();
    await expect(page.getByText("$75.00").first()).toBeVisible();

    // Log out the regular user
    await page.getByRole("button", { name: "Logout" }).click();
    await expect(page).toHaveURL(/\/login/);

    // Step 2: Log in as admin (seeded via `just seed-admin`)
    const admin = getAdminCredentials();
    await loginUser(page, admin);

    // Step 3: Navigate to admin panel
    await page.getByRole("link", { name: "Admin" }).click();
    await expect(page).toHaveURL(/\/admin/);

    // Verify the admin panel loads
    await expect(
      page.getByRole("heading", { name: "Admin Panel" }),
    ).toBeVisible();
    await expect(page.getByText("Registered Users")).toBeVisible();

    // Step 4: Find and click "Assume" on the regular user
    const userRow = page.getByRole("row").filter({
      hasText: regularUser.username,
    });
    await expect(userRow).toBeVisible();
    await userRow.getByRole("button", { name: "Assume" }).click();

    // Step 5: Verify we're now viewing the regular user's dashboard
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByText("User Expense")).toBeVisible();
    await expect(page.getByText("$75.00")).toBeVisible();

    // The "Return to Admin" floating button should be visible
    const returnButton = page.getByRole("button", {
      name: "Return to Admin",
    });
    await expect(returnButton).toBeVisible();

    // Step 6: Click "Return to Admin"
    await returnButton.click();

    // Verify we're back on the admin panel
    await expect(page).toHaveURL(/\/admin/);
    await expect(
      page.getByRole("heading", { name: "Admin Panel" }),
    ).toBeVisible();

    // The "Return to Admin" button should no longer be visible
    await expect(returnButton).not.toBeVisible();
  });
});
