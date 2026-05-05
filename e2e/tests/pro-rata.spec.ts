import { test, expect } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
  logExpense,
} from "../helpers/auth.js";

test.describe("Pro-rata Creation", () => {
  test("logs a pro-rata expense and verifies installments on dashboard", async ({
    page,
  }) => {
    // Setup: register, onboard, create period
    await registerAndOnboard(page, { budget: "3000" });
    await confirmNewPeriod(page);

    // Step 1: Log a pro-rata expense ($300 over 3 months)
    await logExpense(page, {
      name: "Annual Subscription",
      amount: "300",
      type: "essentials",
      proRata: { months: "3" },
    });

    // Step 2: Verify current month shows $100 installment in recent expenses
    // The pro-rata splits $300 into 3 monthly installments of $100
    await expect(page.getByText("Annual Subscription").first()).toBeVisible();
    await expect(page.getByText("$100.00").first()).toBeVisible();

    // Step 3: Verify the "Upcoming Pro-rata" section shows next month's installment
    const upcomingSection = page.getByTestId("upcoming-prorata");
    await expect(upcomingSection).toBeVisible();
    await expect(
      upcomingSection.getByText("Annual Subscription"),
    ).toBeVisible();
    await expect(upcomingSection.getByText("$100.00")).toBeVisible();

    // Verify it shows the installment index (e.g., "Installment 2 of 3")
    await expect(upcomingSection.getByText(/Installment 2 of 3/)).toBeVisible();
  });
});
