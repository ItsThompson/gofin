import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { ExpenseLogFeature } from "../index";
import type { User } from "@gofin/core";
import type { Expense, Tag, BudgetPeriod } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return { ...actual, useNavigate: () => vi.fn() };
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
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
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

function generateExpenses(count: number): Expense[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `exp-${index}`,
    userId: "user-1",
    name: `Expense ${index + 1}`,
    transactionCurrency: "USD",
    transactionAmount: 1000 + index * 100,
    reportingAmount: 1000 + index * 100,
    reportingCurrency: "USD",
    expenseType: "essentials" as const,
    tagId: "tag-food",
    expenseDate: "2026-05-01",
    periodYear: 2026,
    periodMonth: 5,
    status: "active" as const,
    isProRata: false,
    createdAt: `2026-05-01T${String(index).padStart(2, "0")}:00:00Z`,
  }));
}

function mockAllData(expenses: Expense[]) {
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

function renderExpenseLog() {
  return render(
    <MemoryRouter>
      <ExpenseLogFeature user={mockUser} />
    </MemoryRouter>,
  );
}

describe("ExpenseLogFeature - Pagination navigation", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-05-12T12:00:00Z"));
    mockFetch.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("navigates to last page, then back to first via First page button", async () => {
    const expenses = generateExpenses(25);
    mockAllData(expenses);
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getByText("25 expenses")).toBeInTheDocument();
    });

    expect(screen.getByText("1–10 of 25")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Last page"));
    expect(screen.getByText("21–25 of 25")).toBeInTheDocument();

    await user.click(screen.getByLabelText("First page"));
    expect(screen.getByText("1–10 of 25")).toBeInTheDocument();
  });

  it("navigates forward then back via Previous page button", async () => {
    const expenses = generateExpenses(25);
    mockAllData(expenses);
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    renderExpenseLog();

    await waitFor(() => {
      expect(screen.getByText("25 expenses")).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText("Next page"));
    expect(screen.getByText("11–20 of 25")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Previous page"));
    expect(screen.getByText("1–10 of 25")).toBeInTheDocument();
  });
});
