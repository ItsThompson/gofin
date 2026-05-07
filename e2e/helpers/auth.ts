import { type Page, expect } from "@playwright/test";
import { createTestUser } from "./test-user.js";

interface TestUser {
  username: string;
  email: string;
  password: string;
}

/**
 * Register a new user via the UI and return their credentials.
 * After registration, the browser is on the /onboarding page.
 */
export async function registerUser(page: Page): Promise<TestUser> {
  const user = createTestUser();

  await page.goto("/register");
  await page.getByLabel("Username").fill(user.username);
  await page.getByLabel("Email").fill(user.email);
  await page.getByLabel("Password", { exact: true }).fill(user.password);
  await page.getByLabel("Confirm Password").fill(user.password);
  await page.getByRole("button", { name: "Create account" }).click();

  await page.waitForURL("**/onboarding");

  return user;
}

/**
 * Complete the full onboarding wizard with default values.
 * Assumes the page is already on /onboarding.
 * After completion, the browser is on /dashboard.
 */
export async function completeOnboarding(
  page: Page,
  options: { budget?: string } = {},
) {
  // Step 1: Welcome
  await page.getByRole("button", { name: "Get started" }).click();

  // Step 2: Currency (accept default USD)
  await page.getByRole("button", { name: "Continue" }).click();

  // Step 3: Budget
  if (options.budget) {
    await page.getByLabel("Budget Amount").fill(options.budget);
  }
  await page.getByRole("button", { name: "Continue" }).click();

  // Step 4: E/D/S Split (accept defaults 50/30/20)
  await page.getByRole("button", { name: "Complete Setup" }).click();

  await page.waitForURL("**/dashboard");
}

/**
 * Log in with existing credentials via the UI.
 * After login, the browser is on /dashboard (or /onboarding if first time).
 */
export async function loginUser(
  page: Page,
  credentials: { email: string; password: string },
) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(credentials.email);
  await page.getByLabel("Password").fill(credentials.password);
  await page.getByRole("button", { name: "Sign in" }).click();

  // Wait for navigation away from login
  await expect(page).not.toHaveURL(/\/login/);
}

/**
 * Register a new user and complete onboarding in one step.
 * Returns the user credentials. Browser ends on /dashboard.
 */
export async function registerAndOnboard(
  page: Page,
  options: { budget?: string } = {},
): Promise<TestUser> {
  const user = await registerUser(page);
  await completeOnboarding(page, options);
  return user;
}

/**
 * Confirm a new budget period on the dashboard.
 * Called when the dashboard shows the "Set Up [Month]" prompt.
 * Accepts the pre-filled defaults and clicks "Create [Month] Period".
 */
export async function confirmNewPeriod(page: Page) {
  // Wait for the period creation form to appear
  // CardTitle renders a <div>, not a heading element, so use getByText
  await expect(page.getByText(/Set Up .+ \d{4}/)).toBeVisible();

  // Click the create button (text is "Create [MonthName] Period")
  await page.getByRole("button", { name: /Create .+ Period/ }).click();

  // Wait for dashboard to load with active state
  await expect(page.getByText("Total Budget")).toBeVisible();
}

/**
 * Log an expense via the /expenses/new form.
 * Browser should already be authenticated. Navigates to /expenses/new,
 * fills the form, and submits. Ends on /dashboard after redirect.
 */
export async function logExpense(
  page: Page,
  expense: {
    name: string;
    amount: string;
    type?: "essentials" | "desires" | "savings";
    proRata?: { months: string };
  },
) {
  await page.goto("/expenses/new");

  await page.getByLabel("Name").fill(expense.name);
  await page.getByLabel("Amount").fill(expense.amount);

  if (expense.type) {
    await page.getByLabel(capitalize(expense.type)).check();
  }

  if (expense.proRata) {
    await page.getByLabel("Spread across months").check();
    await page.getByLabel("Number of months").fill(expense.proRata.months);
  }

  await page.getByRole("button", { name: "Log Expense" }).click();
  await page.waitForURL("**/dashboard");
}

function capitalize(str: string): string {
  return str.charAt(0).toUpperCase() + str.slice(1);
}
