import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useExpenseLogData } from "../hooks/useExpenseLogData";
import type { FilterCriteria } from "../hooks/useExpenseFilters";
import type { Expense, Tag, BudgetPeriod } from "@gofin/core";

// Mock the API module
vi.mock("../api", () => ({
  expenseLogApi: {
    getExpenses: vi.fn(),
    getTags: vi.fn(),
    getPeriods: vi.fn(),
  },
}));

// Mock useApiToast to pass through calls directly
vi.mock("@gofin/api", () => ({
  useApiToast: () => ({
    call: async (fn: () => Promise<unknown>) => {
      try {
        return await fn();
      } catch {
        return null;
      }
    },
  }),
}));

import { expenseLogApi } from "../api";

const mockGetExpenses = vi.mocked(expenseLogApi.getExpenses);
const mockGetTags = vi.mocked(expenseLogApi.getTags);
const mockGetPeriods = vi.mocked(expenseLogApi.getPeriods);

const emptyFilters: FilterCriteria = {
  selectedTypes: new Set(),
  selectedTags: new Set(),
  selectedTransactionCurrencies: new Set(),
  selectedReportingCurrencies: new Set(),
  dateFrom: "",
  dateTo: "",
};

const mockExpenses: Expense[] = [
  {
    id: "exp-1",
    userId: "user-1",
    name: "Groceries",
    transactionCurrency: "USD",
    transactionAmount: 5000,
    reportingAmount: 5000,
    reportingCurrency: "USD",
    expenseType: "essentials",
    tagId: "tag-food",
    expenseDate: "2026-05-02",
    periodYear: 2026,
    periodMonth: 5,
    status: "active",
    isProRata: false,
    createdAt: "2026-05-02T10:00:00Z",
  },
];

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

function mockSuccessfulFetch(
  expenses: Expense[] = mockExpenses,
  tags: Tag[] = mockTags,
  periods: BudgetPeriod[] = mockPeriods,
) {
  mockGetExpenses.mockResolvedValue({ data: expenses, total: expenses.length, page: 1, pageSize: 1000, hasMore: false });
  mockGetTags.mockResolvedValue({ tags });
  mockGetPeriods.mockResolvedValue({ periods });
}

describe("useExpenseLogData", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-05-12T12:00:00Z"));
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("initial state", () => {
    it("starts with loading state", () => {
      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      expect(result.current.state).toEqual({ status: "loading" });
    });

    it("initializes selection to current year/month", () => {
      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      const now = new Date();
      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      expect(result.current.selectedYear).toBe(now.getFullYear());
      expect(result.current.selectedMonth).toBe(now.getMonth() + 1);
    });
  });

  describe("active state", () => {
    it("replaces all fetch data atomically on success", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      if (result.current.state.status !== "active") {
        throw new Error("expected active state");
      }

      expect(result.current.state.tags).toEqual(mockTags);
      expect(result.current.state.periods).toEqual(mockPeriods);
      expect(result.current.state.expenses).toHaveLength(1);
      expect(result.current.state.expenses[0].name).toBe("Groceries");
      expect(result.current.state.reportingCurrency).toBe("USD");
    });
  });

  describe("error state", () => {
    it("sets error when the fetch fails", async () => {
      mockGetExpenses.mockRejectedValue(new Error("Network error"));
      mockGetTags.mockRejectedValue(new Error("Network error"));
      mockGetPeriods.mockRejectedValue(new Error("Network error"));

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("error");
      });

      if (result.current.state.status !== "error") {
        throw new Error("expected error state");
      }

      expect(result.current.state.message).toBe("Failed to load expense log.");
    });
  });

  describe("missing state", () => {
    it("sets missing when the selected period is absent", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      act(() => {
        result.current.setSelectedMonth(4);
      });

      await waitFor(() => {
        expect(result.current.state.status).toBe("missing");
      });

      if (result.current.state.status !== "missing") {
        throw new Error("expected missing state");
      }

      expect(result.current.state.periods).toEqual(mockPeriods);
    });
  });

  describe("loading transition on selection change", () => {
    it("sets loading when selectedYear changes", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      act(() => {
        result.current.setSelectedYear(2025);
      });

      expect(result.current.state.status).toBe("loading");
    });

    it("sets loading when selectedMonth changes", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      act(() => {
        result.current.setSelectedMonth(3);
      });

      expect(result.current.state.status).toBe("loading");
    });

    it("transitions to active after a successful fetch for the new period", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      const aprilExpenses: Expense[] = [{
        ...mockExpenses[0],
        id: "exp-apr-1",
        name: "April Purchase",
        periodMonth: 4,
      }];
      const aprilPeriods: BudgetPeriod[] = [
        {
          ...mockPeriods[0],
          id: "period-apr",
          month: 4,
        },
      ];
      mockSuccessfulFetch(aprilExpenses, mockTags, aprilPeriods);

      act(() => {
        result.current.setSelectedMonth(4);
      });

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      if (result.current.state.status !== "active") {
        throw new Error("expected active state");
      }

      expect(result.current.state.expenses[0].name).toBe("April Purchase");
    });
  });

  describe("selection state", () => {
    it("selectedYear and selectedMonth remain as individual atoms", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      act(() => {
        result.current.setSelectedYear(2025);
      });

      expect(result.current.selectedYear).toBe(2025);
      expect(result.current.selectedMonth).toBe(new Date().getMonth() + 1);

      mockSuccessfulFetch();
      act(() => {
        result.current.setSelectedMonth(3);
      });

      expect(result.current.selectedMonth).toBe(3);
    });
  });

  describe("public interface", () => {
    it("exposes all ExpenseLogData fields", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.state.status).toBe("active");
      });

      expect(result.current).toHaveProperty("state");
      expect(result.current).toHaveProperty("selectedYear");
      expect(result.current).toHaveProperty("selectedMonth");
      expect(result.current).toHaveProperty("setSelectedYear");
      expect(result.current).toHaveProperty("setSelectedMonth");
      expect(result.current).toHaveProperty("refresh");

      expect(typeof result.current.setSelectedYear).toBe("function");
      expect(typeof result.current.setSelectedMonth).toBe("function");
      expect(typeof result.current.refresh).toBe("function");
    });
  });
});
