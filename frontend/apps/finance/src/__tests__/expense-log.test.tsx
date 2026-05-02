import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { ExpenseLogPage } from "@/pages/ExpenseLogPage";
import type { User, Expense, Tag, BudgetPeriod } from "@gofin/types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

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
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z" },
  { id: "tag-transport", name: "Transport", isDefault: true, createdAt: "2026-01-01T00:00:00Z" },
  { id: "tag-bills", name: "Bills", isDefault: true, createdAt: "2026-01-01T00:00:00Z" },
];

const mockPeriods: BudgetPeriod[] = [
  {
    id: "period-may",
    userId: "user-1",
    year: 2026,
    month: 5,
    budgetAmount: 300000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
  {
    id: "period-apr",
    userId: "user-1",
    year: 2026,
    month: 4,
    budgetAmount: 250000,
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-04-01T00:00:00Z",
    updatedAt: "2026-04-01T00:00:00Z",
  },
];

const mockExpenses: Expense[] = [
  {
    id: "exp-1",
    userId: "user-1",
    name: "Groceries",
    amount: 5000,
    currency: "USD",
    expenseType: "essentials",
    tagId: "tag-food",
    expenseDate: "2026-05-02",
    periodYear: 2026,
    periodMonth: 5,
    status: "active",
    isProRata: false,
    createdAt: "2026-05-02T10:00:00Z",
  },
  {
    id: "exp-2",
    userId: "user-1",
    name: "Bus Pass",
    amount: 2000,
    currency: "USD",
    expenseType: "essentials",
    tagId: "tag-transport",
    expenseDate: "2026-05-01",
    periodYear: 2026,
    periodMonth: 5,
    status: "active",
    isProRata: false,
    createdAt: "2026-05-01T09:00:00Z",
  },
  {
    id: "exp-3",
    userId: "user-1",
    name: "Old Coffee",
    amount: 450,
    currency: "USD",
    expenseType: "desires",
    tagId: "tag-food",
    expenseDate: "2026-05-01",
    periodYear: 2026,
    periodMonth: 5,
    status: "corrected",
    isProRata: false,
    createdAt: "2026-05-01T08:00:00Z",
  },
];

/**
 * Mock the three parallel API requests: expenses, tags, periods.
 * The fetch order is: expenses first, then tags, then periods
 * (due to Promise.all ordering).
 */
function mockAllDataSuccess(
  expenses: Expense[] = mockExpenses,
  tags: Tag[] = mockTags,
  periods: BudgetPeriod[] = mockPeriods,
) {
  // Expenses request
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
  // Tags request
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ tags }),
  });
  // Periods request
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ periods }),
  });
}

function mockEmptyExpenses(
  tags: Tag[] = mockTags,
  periods: BudgetPeriod[] = mockPeriods,
) {
  mockAllDataSuccess([], tags, periods);
}

function renderExpenseLog(user: User = mockUser) {
  return render(
    <MemoryRouter>
      <ExpenseLogPage user={user} />
    </MemoryRouter>,
  );
}

function renderExpenseLogWithSearchParams(
  searchParams: string,
  user: User = mockUser,
) {
  return render(
    <MemoryRouter initialEntries={[`/expenses?${searchParams}`]}>
      <ExpenseLogPage user={user} />
    </MemoryRouter>,
  );
}

describe("ExpenseLogPage", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
  });

  it("renders loading state initially", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => {}));
    renderExpenseLog();
    expect(screen.getByText("Loading expenses...")).toBeInTheDocument();
  });

  describe("table rendering", () => {
    it("renders expense table with correct columns", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      // Column headers
      expect(screen.getByText("Date")).toBeInTheDocument();
      expect(screen.getByText("Name")).toBeInTheDocument();
      expect(screen.getByText("Amount")).toBeInTheDocument();
      expect(screen.getByText("Type")).toBeInTheDocument();
      expect(screen.getByText("Tag")).toBeInTheDocument();
      expect(screen.getByText("Status")).toBeInTheDocument();
    });

    it("renders expense data with resolved tag names", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        // Appears in both desktop table and mobile list
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      expect(screen.getAllByText("Bus Pass").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("Old Coffee").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("$50.00").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("$20.00").length).toBeGreaterThanOrEqual(1);

      // Tag names resolved (appear in desktop table)
      expect(screen.getAllByText("Food").length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText("Transport").length).toBeGreaterThanOrEqual(1);
    });

    it("renders formatted currency amounts", async () => {
      mockAllDataSuccess();
      renderExpenseLog({ ...mockUser, currency: "EUR" });

      await waitFor(() => {
        expect(screen.getAllByText("€50.00").length).toBeGreaterThanOrEqual(1);
      });

      expect(screen.getAllByText("€20.00").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("€4.50").length).toBeGreaterThanOrEqual(1);
    });

    it("shows total expense count", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("3 expenses")).toBeInTheDocument();
      });
    });
  });

  describe("corrected expenses", () => {
    it("displays strikethrough on corrected expense text", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Old Coffee").length).toBeGreaterThanOrEqual(1);
      });

      // At least one instance of "Old Coffee" should have line-through class
      const correctedNames = screen.getAllByText("Old Coffee");
      const hasStrikethrough = correctedNames.some((element) =>
        element.className.includes("line-through"),
      );
      expect(hasStrikethrough).toBe(true);
    });

    it("shows 'Corrected' badge in status column", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Corrected").length).toBeGreaterThanOrEqual(1);
      });
    });

    it("shows 'Active' badge for active expenses", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        // Two active expenses
        expect(screen.getAllByText("Active")).toHaveLength(2);
      });
    });
  });

  describe("sorting", () => {
    it("sorts by column when clicking header", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      // Click "Name" header to sort ascending
      const nameHeader = screen.getByText("Name");
      await user.click(nameHeader);

      // Check table rows in the desktop table
      const tableElement = document.querySelector("table")!;
      const tableRows = tableElement.querySelectorAll("tbody tr");
      expect(tableRows[0].textContent).toContain("Bus Pass");
    });

    it("toggles sort direction on repeated clicks", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      // Click "Date" header to sort ascending (string sort, reliable)
      const dateHeader = screen.getByText("Date");

      // Click once: ascending by date
      await user.click(dateHeader);
      const tableElement = document.querySelector("table")!;
      let tableRows = tableElement.querySelectorAll("tbody tr");
      // Ascending date: 2026-05-01 before 2026-05-02
      expect(tableRows[0].textContent).toContain("Bus Pass");

      // Click again: descending
      await user.click(dateHeader);
      tableRows = tableElement.querySelectorAll("tbody tr");
      // Descending date: 2026-05-02 first
      expect(tableRows[0].textContent).toContain("Groceries");
    });
  });

  describe("filtering", () => {
    it("shows filter panel when Filters button is clicked", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      const filtersButton = screen.getByRole("button", { name: /filters/i });
      await user.click(filtersButton);

      expect(screen.getByText("Expense Type")).toBeInTheDocument();
      expect(screen.getByText("Date Range")).toBeInTheDocument();
    });

    it("filters by expense type", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      // Open filters
      await user.click(screen.getByRole("button", { name: /filters/i }));

      // Click "desires" type filter
      await user.click(screen.getByRole("button", { name: "desires" }));

      // Only "Old Coffee" (desires type) should be visible
      expect(screen.getAllByText("Old Coffee").length).toBeGreaterThanOrEqual(1);
      expect(screen.queryByText("Groceries")).not.toBeInTheDocument();
      expect(screen.queryByText("Bus Pass")).not.toBeInTheDocument();
      expect(screen.getByText("1 expense")).toBeInTheDocument();
    });

    it("filters by tag", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      await user.click(screen.getByRole("button", { name: /filters/i }));

      // Filter by "Transport" tag
      await user.click(screen.getByRole("button", { name: "Transport" }));

      expect(screen.getAllByText("Bus Pass").length).toBeGreaterThanOrEqual(1);
      expect(screen.queryByText("Groceries")).not.toBeInTheDocument();
      expect(screen.getByText("1 expense")).toBeInTheDocument();
    });

    it("applies combinatorial AND logic for filters", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      await user.click(screen.getByRole("button", { name: /filters/i }));

      // Filter by essentials type AND transport tag
      await user.click(screen.getByRole("button", { name: "essentials" }));
      await user.click(screen.getByRole("button", { name: "Transport" }));

      // Only Bus Pass matches both criteria
      expect(screen.getAllByText("Bus Pass").length).toBeGreaterThanOrEqual(1);
      expect(screen.queryByText("Groceries")).not.toBeInTheDocument();
      expect(screen.getByText("1 expense")).toBeInTheDocument();
    });

    it("filters by date range", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      await user.click(screen.getByRole("button", { name: /filters/i }));

      // Set date from to 2026-05-02 (should exclude May 1st expenses)
      const dateFromInput = screen.getByLabelText("Date from");
      await user.type(dateFromInput, "2026-05-02");

      // Only Groceries (May 2) should remain
      expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      expect(screen.queryByText("Bus Pass")).not.toBeInTheDocument();
      expect(screen.queryByText("Old Coffee")).not.toBeInTheDocument();
    });

    it("clears all filters when Clear filters is clicked", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      await user.click(screen.getByRole("button", { name: /filters/i }));
      await user.click(screen.getByRole("button", { name: "desires" }));

      // Only 1 expense shown
      expect(screen.getByText("1 expense")).toBeInTheDocument();

      // Clear
      await user.click(screen.getByRole("button", { name: /clear filters/i }));

      // All 3 expenses back
      expect(screen.getByText("3 expenses")).toBeInTheDocument();
    });
  });

  describe("pagination", () => {
    it("renders pagination controls", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      expect(screen.getByLabelText("Page size")).toBeInTheDocument();
      expect(screen.getByLabelText("First page")).toBeInTheDocument();
      expect(screen.getByLabelText("Previous page")).toBeInTheDocument();
      expect(screen.getByLabelText("Next page")).toBeInTheDocument();
      expect(screen.getByLabelText("Last page")).toBeInTheDocument();
    });

    it("shows correct range text", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      // Default page size is 10, 3 expenses total
      expect(screen.getByText("1–3 of 3")).toBeInTheDocument();
    });

    it("paginates data when page size is smaller than total", async () => {
      // Create 15 expenses to test pagination
      const manyExpenses: Expense[] = Array.from({ length: 15 }, (_, index) => ({
        id: `exp-${index}`,
        userId: "user-1",
        name: `Expense ${index + 1}`,
        amount: 1000 + index * 100,
        currency: "USD",
        expenseType: "essentials" as const,
        tagId: "tag-food",
        expenseDate: `2026-05-${String(index + 1).padStart(2, "0")}`,
        periodYear: 2026,
        periodMonth: 5,
        status: "active",
        isProRata: false,
        createdAt: `2026-05-${String(index + 1).padStart(2, "0")}T10:00:00Z`,
      }));

      mockAllDataSuccess(manyExpenses);
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("15 expenses")).toBeInTheDocument();
      });

      // Default page size 10: showing 1-10 of 15
      expect(screen.getByText("1–10 of 15")).toBeInTheDocument();

      // Navigate to next page
      await user.click(screen.getByLabelText("Next page"));
      expect(screen.getByText("11–15 of 15")).toBeInTheDocument();
    });

    it("allows changing page size", async () => {
      const manyExpenses: Expense[] = Array.from({ length: 30 }, (_, index) => ({
        id: `exp-${index}`,
        userId: "user-1",
        name: `Expense ${index + 1}`,
        amount: 1000,
        currency: "USD",
        expenseType: "essentials" as const,
        tagId: "tag-food",
        expenseDate: "2026-05-01",
        periodYear: 2026,
        periodMonth: 5,
        status: "active",
        isProRata: false,
        createdAt: "2026-05-01T10:00:00Z",
      }));

      mockAllDataSuccess(manyExpenses);
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("30 expenses")).toBeInTheDocument();
      });

      // Change to 25 per page
      await user.selectOptions(screen.getByLabelText("Page size"), "25");
      expect(screen.getByText("1–25 of 30")).toBeInTheDocument();
    });
  });

  describe("period switching", () => {
    it("renders period selector with available periods", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      const periodSelect = screen.getByLabelText("Period:");
      expect(periodSelect).toBeInTheDocument();

      // Both periods should be options
      const options = within(periodSelect).getAllByRole("option");
      expect(options).toHaveLength(2);
      expect(options[0].textContent).toContain("May");
      expect(options[1].textContent).toContain("April");
    });

    it("reloads data when period is changed", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      // Switch to April
      const aprilExpenses: Expense[] = [
        {
          id: "exp-apr-1",
          userId: "user-1",
          name: "April Rent",
          amount: 100000,
          currency: "USD",
          expenseType: "essentials",
          tagId: "tag-bills",
          expenseDate: "2026-04-01",
          periodYear: 2026,
          periodMonth: 4,
          status: "active",
          isProRata: false,
          createdAt: "2026-04-01T10:00:00Z",
        },
      ];
      mockAllDataSuccess(aprilExpenses);

      fireEvent.change(screen.getByLabelText("Period:"), {
        target: { value: "2026-4" },
      });

      await waitFor(() => {
        expect(screen.getAllByText("April Rent").length).toBeGreaterThanOrEqual(1);
      });

      expect(screen.queryByText("Groceries")).not.toBeInTheDocument();
    });
  });

  describe("row click navigation", () => {
    it("navigates to expense detail on row click", async () => {
      mockAllDataSuccess();
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      // Click the first table row containing "Groceries"
      const tableElement = document.querySelector("table")!;
      const tableRows = tableElement.querySelectorAll("tbody tr");
      const groceriesRow = Array.from(tableRows).find((row) =>
        row.textContent?.includes("Groceries"),
      );
      expect(groceriesRow).toBeDefined();
      await user.click(groceriesRow!);

      expect(mockNavigate).toHaveBeenCalledWith("/expenses/exp-1");
    });
  });

  describe("empty state", () => {
    it("shows empty state when no expenses exist for the period", async () => {
      mockEmptyExpenses();
      renderExpenseLog();

      await waitFor(() => {
        expect(
          screen.getByText("No expenses for this period"),
        ).toBeInTheDocument();
      });
    });

    it("shows 0 expenses count in header", async () => {
      mockEmptyExpenses();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("0 expenses")).toBeInTheDocument();
      });
    });
  });

  describe("mobile responsive layout", () => {
    it("renders both desktop table and mobile list (CSS handles visibility)", async () => {
      mockAllDataSuccess();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      // Both layouts should be in the DOM (CSS hides one or the other)
      // Desktop table has <table> element
      const tables = document.querySelectorAll("table");
      expect(tables.length).toBe(1);

      // Expense names appear twice: once in desktop table, once in mobile list
      const allGroceries = screen.getAllByText("Groceries");
      expect(allGroceries.length).toBe(2);
    });
  });

  describe("error state", () => {
    it("shows error state when all fetches fail", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Error")).toBeInTheDocument();
      });

      expect(
        screen.getByText("Failed to load expenses. Please try again."),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /retry/i }),
      ).toBeInTheDocument();
    });

    it("retries on retry button click", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));
      const user = userEvent.setup();
      renderExpenseLog();

      await waitFor(() => {
        expect(screen.getByText("Error")).toBeInTheDocument();
      });

      // Retry succeeds
      mockAllDataSuccess();
      await user.click(screen.getByRole("button", { name: /retry/i }));

      await waitFor(() => {
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  describe("graceful tag degradation", () => {
    it("shows tag IDs when tags endpoint fails", async () => {
      // Expenses succeed
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            data: mockExpenses,
            total: mockExpenses.length,
            page: 1,
            pageSize: 1000,
            hasMore: false,
          }),
      });
      // Tags fail
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () =>
          Promise.resolve({
            code: "INTERNAL_SERVER_ERROR",
            message: "Database error",
          }),
      });
      // Periods succeed
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ periods: mockPeriods }),
      });

      renderExpenseLog();

      await waitFor(() => {
        // Tag ID is shown as fallback since tags couldn't be resolved
        expect(screen.getAllByText("Groceries").length).toBeGreaterThanOrEqual(1);
      });

      // Tag column should show the raw tag ID (at least in desktop table)
      expect(screen.getAllByText("tag-food").length).toBeGreaterThanOrEqual(2);
    });
  });

  describe("URL search params", () => {
    it("initializes tag filter from ?tag= URL parameter", async () => {
      mockAllDataSuccess();
      renderExpenseLogWithSearchParams("tag=tag-food");

      await waitFor(() => {
        expect(screen.getByText("Expense Log")).toBeInTheDocument();
      });

      // Filter panel should be visible (auto-opened when tag param present)
      expect(screen.getByText("Expense Type")).toBeInTheDocument();

      // Only food-tagged expenses should be shown
      await waitFor(() => {
        expect(screen.getByText("2 expenses")).toBeInTheDocument();
      });
    });
  });
});
