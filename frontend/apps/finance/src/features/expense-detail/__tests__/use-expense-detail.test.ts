import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { useExpenseDetail } from "@/features/expense-detail";
import type { Expense } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const testExpense: Expense = {
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
};

function mockSuccessfulFetch(expense: Expense = testExpense, history: Expense[] = [expense]) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ expense }),
  });
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ entries: history }),
  });
}

function mockFailedFetch() {
  mockFetch.mockRejectedValueOnce(new Error("Network error"));
}

describe("useExpenseDetail", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  describe("loading → detail transition", () => {
    it("starts in loading status", () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      expect(result.current.status).toBe("loading");
    });

    it("transitions to detail status after successful fetch", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      if (result.current.status !== "detail") throw new Error("Expected detail status");
      expect(result.current.expense).toEqual(testExpense);
      expect(result.current.history).toEqual([testExpense]);
      expect(result.current.proRataGroup).toEqual([]);
    });

    it("provides correction state in detail variant", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      if (result.current.status !== "detail") throw new Error("Expected detail status");
      expect(result.current.correction).toBeDefined();
      expect(result.current.correction.submitting).toBe(false);
      expect(result.current.correction.error).toBeNull();
      expect(typeof result.current.correction.submitCorrection).toBe("function");
      expect(typeof result.current.correction.clearError).toBe("function");
    });

    it("provides startCorrection only in detail variant", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      if (result.current.status !== "detail") throw new Error("Expected detail status");
      expect(typeof result.current.startCorrection).toBe("function");
    });

    it("provides refresh on loading variant", () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      expect(result.current.status).toBe("loading");
      expect(typeof result.current.refresh).toBe("function");
    });
  });

  describe("detail → correct → detail transitions", () => {
    it("transitions to correct status when startCorrection is called", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      act(() => {
        if (result.current.status === "detail") {
          result.current.startCorrection();
        }
      });

      expect(result.current.status).toBe("correct");
      if (result.current.status !== "correct") throw new Error("Expected correct status");
      expect(result.current.expense).toEqual(testExpense);
      expect(result.current.history).toEqual([testExpense]);
      expect(result.current.proRataGroup).toEqual([]);
    });

    it("provides cancelCorrection only in correct variant", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      act(() => {
        if (result.current.status === "detail") {
          result.current.startCorrection();
        }
      });

      if (result.current.status !== "correct") throw new Error("Expected correct status");
      expect(typeof result.current.cancelCorrection).toBe("function");
    });

    it("transitions back to detail when cancelCorrection is called", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      act(() => {
        if (result.current.status === "detail") {
          result.current.startCorrection();
        }
      });

      expect(result.current.status).toBe("correct");

      act(() => {
        if (result.current.status === "correct") {
          result.current.cancelCorrection();
        }
      });

      expect(result.current.status).toBe("detail");
      if (result.current.status !== "detail") throw new Error("Expected detail status");
      expect(result.current.expense).toEqual(testExpense);
    });

    it("provides correction state in correct variant", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      act(() => {
        if (result.current.status === "detail") {
          result.current.startCorrection();
        }
      });

      if (result.current.status !== "correct") throw new Error("Expected correct status");
      expect(result.current.correction).toBeDefined();
      expect(result.current.correction.submitting).toBe(false);
      expect(result.current.correction.error).toBeNull();
    });
  });

  describe("loading → error transition", () => {
    it("transitions to error status on fetch failure", async () => {
      mockFailedFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("error");
      });

      if (result.current.status !== "error") throw new Error("Expected error status");
      expect(result.current.error).toBe("Failed to load expense details.");
    });

    it("provides refresh on error variant for retry", async () => {
      mockFailedFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("error");
      });

      if (result.current.status !== "error") throw new Error("Expected error status");
      expect(typeof result.current.refresh).toBe("function");
    });

    it("can recover from error via refresh", async () => {
      mockFailedFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      await waitFor(() => {
        expect(result.current.status).toBe("error");
      });

      // Set up successful fetch for retry
      mockSuccessfulFetch();

      act(() => {
        result.current.refresh();
      });

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      if (result.current.status !== "detail") throw new Error("Expected detail status");
      expect(result.current.expense).toEqual(testExpense);
    });
  });

  describe("type narrowing guarantees", () => {
    it("expense is only accessible in detail and correct variants", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      // Loading: no expense property
      expect("expense" in result.current).toBe(false);

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      // Detail: expense is present and non-null
      if (result.current.status === "detail") {
        expect(result.current.expense).toBeTruthy();
        expect(result.current.expense.id).toBe("exp-1");
      }
    });

    it("startCorrection is not accessible in correct or loading variants", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      // Loading: no startCorrection
      expect("startCorrection" in result.current).toBe(false);

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      act(() => {
        if (result.current.status === "detail") {
          result.current.startCorrection();
        }
      });

      // Correct: no startCorrection
      expect("startCorrection" in result.current).toBe(false);
      expect("cancelCorrection" in result.current).toBe(true);
    });

    it("cancelCorrection is not accessible in detail or loading variants", async () => {
      mockSuccessfulFetch();
      const { result } = renderHook(() => useExpenseDetail("exp-1"));

      // Loading: no cancelCorrection
      expect("cancelCorrection" in result.current).toBe(false);

      await waitFor(() => {
        expect(result.current.status).toBe("detail");
      });

      // Detail: no cancelCorrection
      expect("cancelCorrection" in result.current).toBe(false);
    });
  });

  describe("null expenseId handling", () => {
    it("stays in loading status when expenseId is null", async () => {
      const { result } = renderHook(() => useExpenseDetail(null));

      // Should remain in loading since no fetch is triggered
      expect(result.current.status).toBe("loading");
      expect(mockFetch).not.toHaveBeenCalled();
    });
  });
});
