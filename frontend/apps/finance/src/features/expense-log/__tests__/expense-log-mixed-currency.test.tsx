import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { ExpenseLogFeature } from "../index";
import type { User, Expense, Tag, BudgetPeriod } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockUser: User = {
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockTags: Tag[] = [
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-travel", name: "Travel", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
];

const mockPeriods: BudgetPeriod[] = [
  {
    id: "period-may",
    userId: "user-1",
    year: 2026,
    month: 5,
    budgetAmount: 300000,
    reportingCurrency: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
];

/** Same-currency expense (USD transaction, USD reporting). */
const sameCurrencyExpense: Expense = {
  id: "exp-same",
  userId: "user-1",
  name: "Groceries",
  amount: 5000,
  transactionCurrency: "USD",
  transactionAmount: 5000,
  reportingAmount: 5000,
  reportingCurrency: "USD",
  exchangeRate: "1",
  exchangeRateSource: "identity",
  exchangeRateTimestamp: "2026-05-02T10:00:00Z",
  expenseType: "essentials",
  tagId: "tag-food",
  expenseDate: "2026-05-02",
  periodYear: 2026,
  periodMonth: 5,
  status: "active",
  isProRata: false,
  createdAt: "2026-05-02T10:00:00Z",
};

/** Foreign-currency expense (EUR transaction, USD reporting). */
const foreignCurrencyExpense: Expense = {
  id: "exp-foreign",
  userId: "user-1",
  name: "Hotel",
  amount: 15000,
  transactionCurrency: "EUR",
  transactionAmount: 15000,
  reportingAmount: 16200,
  reportingCurrency: "USD",
  exchangeRate: "1.08",
  exchangeRateSource: "open_exchange_rates",
  exchangeRateTimestamp: "2026-05-01T10:00:00Z",
  expenseType: "desires",
  tagId: "tag-travel",
  expenseDate: "2026-05-01",
  periodYear: 2026,
  periodMonth: 5,
  status: "active",
  isProRata: false,
  createdAt: "2026-05-01T09:00:00Z",
};

/** JPY transaction expense with USD reporting (zero-decimal transaction currency). */
const jpyExpense: Expense = {
  id: "exp-jpy",
  userId: "user-1",
  name: "Tokyo Lunch",
  amount: 2000,
  transactionCurrency: "JPY",
  transactionAmount: 2000,
  reportingAmount: 1350,
  reportingCurrency: "USD",
  exchangeRate: "0.00675",
  exchangeRateSource: "open_exchange_rates",
  exchangeRateTimestamp: "2026-05-03T10:00:00Z",
  expenseType: "essentials",
  tagId: "tag-food",
  expenseDate: "2026-05-03",
  periodYear: 2026,
  periodMonth: 5,
  status: "active",
  isProRata: false,
  createdAt: "2026-05-03T10:00:00Z",
};

function mockAllDataSuccess(expenses: Expense[] = [sameCurrencyExpense, foreignCurrencyExpense]) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        data: expenses,
        total: expenses.length,
        page: 1,
        pageSize: 1000,
        hasMore: false,
      }),
  });
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ tags: mockTags }),
  });
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ periods: mockPeriods }),
  });
}

function renderExpenseLog(user: User = mockUser) {
  return render(
    <MemoryRouter>
      <ExpenseLogFeature user={user} />
    </MemoryRouter>,
  );
}

describe("ExpenseLogFeature mixed-currency", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("same-currency row shows one amount", async () => {
    mockAllDataSuccess([sameCurrencyExpense]);
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
    });

    // The amount column should show $50.00 (transaction = reporting)
    expect(screen.getAllByText("$50.00").length).toBeGreaterThanOrEqual(1);
  });

  it("foreign-currency row shows transaction amount and secondary reporting amount", async () => {
    mockAllDataSuccess([foreignCurrencyExpense]);
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Hotel").length).toBeGreaterThanOrEqual(1);
    });

    // Transaction amount formatted in EUR
    expect(screen.getAllByText("€150.00").length).toBeGreaterThanOrEqual(1);
    // Secondary reporting amount formatted in USD with a distinguishing label
    expect(screen.getAllByText("Budget impact: $162.00").length).toBeGreaterThanOrEqual(1);
  });

  it("JPY transaction shows zero-decimal transaction amount and USD reporting amount", async () => {
    mockAllDataSuccess([jpyExpense]);
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Tokyo Lunch").length).toBeGreaterThanOrEqual(1);
    });

    // JPY has 0 minor unit digits
    expect(screen.getAllByText("¥2,000").length).toBeGreaterThanOrEqual(1);
    // USD reporting amount
    expect(screen.getAllByText("Budget impact: $13.50").length).toBeGreaterThanOrEqual(1);
  });

  it("sorts by reporting amount when clicking Amount header", async () => {
    // exp-foreign has reportingAmount 16200, exp-same has reportingAmount 5000.
    // Data order is [foreign, same] = [Hotel, Groceries].
    // After ascending sort by reportingAmount, same (5000) should come before foreign (16200).
    mockAllDataSuccess([foreignCurrencyExpense, sameCurrencyExpense]);
    const user = userEvent.setup();
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
    });

    // Click the Amount header button to sort ascending by reporting amount.
    const amountHeader = screen.getByText("Amount").closest("button")!;
    await user.click(amountHeader);

    const tableElement = document.querySelector("table")!;
    let tableRows = tableElement.querySelectorAll("tbody tr");
    // Ascending by reportingAmount: Groceries (5000) before Hotel (16200)
    await waitFor(() => {
      tableRows = tableElement.querySelectorAll("tbody tr");
      expect(tableRows[0].textContent).toContain("Groceries");
    });
    expect(tableRows[1].textContent).toContain("Hotel");

    // Descending: Hotel (16200) before Groceries (5000)
    await user.click(amountHeader);
    const updatedRows = tableElement.querySelectorAll("tbody tr");
    expect(updatedRows[0].textContent).toContain("Hotel");
    expect(updatedRows[1].textContent).toContain("Groceries");
  });

  it("filters by transaction currency", async () => {
    mockAllDataSuccess([sameCurrencyExpense, foreignCurrencyExpense]);
    const user = userEvent.setup();
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
    });

    await user.click(screen.getByRole("button", { name: /filters/i }));

    // Click EUR in the Transaction Currency filter section.
    const transactionCurrencySection = screen.getByText("Transaction Currency").closest("div")!;
    const eurButton = within(transactionCurrencySection).getByRole("button", { name: "EUR" });
    await user.click(eurButton);

    // Only the EUR transaction expense should be visible
    expect(screen.getAllByText("Hotel").length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText("Groceries")).not.toBeInTheDocument();
  });

  it("filters by reporting currency", async () => {
    mockAllDataSuccess([sameCurrencyExpense, foreignCurrencyExpense]);
    const user = userEvent.setup();
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
    });

    await user.click(screen.getByRole("button", { name: /filters/i }));

    // Click USD in the Reporting Currency filter section -- both rows have USD reporting
    const reportingCurrencySection = screen.getByText("Reporting Currency").closest("div")!;
    const usdButton = within(reportingCurrencySection).getByRole("button", { name: "USD" });
    await user.click(usdButton);

    // Both should still be visible since both report in USD
    expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Hotel").length).toBeGreaterThanOrEqual(1);
  });

  it("applies AND logic for transaction and reporting currency filters", async () => {
    mockAllDataSuccess([sameCurrencyExpense, foreignCurrencyExpense]);
    const user = userEvent.setup();
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
    });

    await user.click(screen.getByRole("button", { name: /filters/i }));

    // Filter by EUR transaction currency AND USD reporting currency
    const transactionCurrencySection = screen.getByText("Transaction Currency").closest("div")!;
    const eurButton = within(transactionCurrencySection).getByRole("button", { name: "EUR" });
    await user.click(eurButton);

    const reportingCurrencySection = screen.getByText("Reporting Currency").closest("div")!;
    const usdButton = within(reportingCurrencySection).getByRole("button", { name: "USD" });
    await user.click(usdButton);

    // Only the EUR/USD expense (Hotel) matches both filters
    expect(screen.getAllByText("Hotel").length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText("Groceries")).not.toBeInTheDocument();
  });

  it("mobile list preserves secondary reporting amount for mixed-currency rows", async () => {
    mockAllDataSuccess([foreignCurrencyExpense]);
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getAllByText("Hotel").length).toBeGreaterThanOrEqual(1);
    });

    // Both the desktop table and mobile list are in the DOM (CSS controls
    // visibility). For a single foreign expense, the labeled secondary amount
    // must appear once in the desktop table and once in the mobile list.
    const mobileList = document.querySelector(".md\\:hidden");
    expect(mobileList).not.toBeNull();
    const mobileReportingAmounts = mobileList
      ? within(mobileList as HTMLElement).getAllByText("Budget impact: $162.00")
      : [];
    expect(mobileReportingAmounts.length).toBe(1);

    // The desktop table also renders the secondary amount.
    const desktopReportingAmounts = screen.getAllByText("Budget impact: $162.00");
    expect(desktopReportingAmounts.length).toBeGreaterThanOrEqual(2);
  });
});