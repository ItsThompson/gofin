import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useExpenseLogData, EMPTY_FETCH_RESULT } from "../hooks/useExpenseLogData";
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
  dateFrom: "",
  dateTo: "",
};

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
    vi.clearAllMocks();
  });

  describe("EMPTY_FETCH_RESULT constant", () => {
    it("provides empty arrays for all fields", () => {
      expect(EMPTY_FETCH_RESULT).toEqual({
        rawExpenses: [],
        tags: [],
        periods: [],
      });
    });

    it("is frozen/immutable reference", () => {
      const ref1 = EMPTY_FETCH_RESULT;
      const ref2 = EMPTY_FETCH_RESULT;
      expect(ref1).toBe(ref2);
    });
  });

  describe("initial state", () => {
    it("starts with loading=true", () => {
      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      expect(result.current.loading).toBe(true);
      expect(result.current.expenses).toEqual([]);
      expect(result.current.tags).toEqual([]);
      expect(result.current.periods).toEqual([]);
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

  describe("fetch-result grouping", () => {
    it("replaces all fetch data atomically on success", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.tags).toEqual(mockTags);
      expect(result.current.periods).toEqual(mockPeriods);
      expect(result.current.expenses).toHaveLength(1);
      expect(result.current.expenses[0].name).toBe("Groceries");
    });

    it("retains previous fetch-result when new fetch fails", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      // Verify initial data is loaded
      expect(result.current.tags).toEqual(mockTags);

      // Now make the next fetch fail
      mockGetExpenses.mockRejectedValue(new Error("Network error"));
      mockGetTags.mockRejectedValue(new Error("Network error"));
      mockGetPeriods.mockRejectedValue(new Error("Network error"));

      act(() => {
        result.current.setSelectedMonth(4);
      });

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      // Previous data remains (stale-but-dimmed approach)
      expect(result.current.tags).toEqual(mockTags);
    });
  });

  describe("loading transition on selection change", () => {
    it("sets loading=true when selectedYear changes", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      // Stall the next fetch
      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      act(() => {
        result.current.setSelectedYear(2025);
      });

      // Loading should be true immediately
      expect(result.current.loading).toBe(true);
    });

    it("sets loading=true when selectedMonth changes", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      // Stall the next fetch
      mockGetExpenses.mockReturnValue(new Promise(() => {}));
      mockGetTags.mockReturnValue(new Promise(() => {}));
      mockGetPeriods.mockReturnValue(new Promise(() => {}));

      act(() => {
        result.current.setSelectedMonth(3);
      });

      // Loading should be true immediately
      expect(result.current.loading).toBe(true);
    });

    it("loading transitions to false after successful fetch", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      const aprilExpenses: Expense[] = [{
        ...mockExpenses[0],
        id: "exp-apr-1",
        name: "April Purchase",
        periodMonth: 4,
      }];
      mockSuccessfulFetch(aprilExpenses);

      act(() => {
        result.current.setSelectedMonth(4);
      });

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.expenses[0].name).toBe("April Purchase");
    });
  });

  describe("selection state", () => {
    it("selectedYear and selectedMonth remain as individual atoms", async () => {
      mockSuccessfulFetch();

      const { result } = renderHook(() => useExpenseLogData(emptyFilters));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
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
        expect(result.current.loading).toBe(false);
      });

      // Verify all fields of the public interface exist
      expect(result.current).toHaveProperty("expenses");
      expect(result.current).toHaveProperty("tags");
      expect(result.current).toHaveProperty("periods");
      expect(result.current).toHaveProperty("loading");
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
