import { describe, it, expect, beforeEach, vi } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { usePeriodState } from "../hooks/usePeriodState";
import { createMockApi, buildPeriod, buildDefaults } from "@gofin/test-utils";
import type { PeriodStateResult } from "../types";

const testPeriod = buildPeriod({
  id: "period-abc",
  userId: "user-1",
  year: 2026,
  month: 5,
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
});

const testDefaults = buildDefaults({
  userId: "user-1",
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
});

describe("usePeriodState", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts in loading status", () => {
    global.fetch = (() => new Promise(() => {})) as unknown as typeof fetch;
    const { result } = renderHook(() => usePeriodState());

    expect(result.current.status).toBe("loading");
    expect(result.current.retry).toBeInstanceOf(Function);
  });

  describe("loading → active transition", () => {
    it("transitions to active with period data on successful fetch", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      expect(result.current.status).toBe("loading");

      await waitFor(() => {
        expect(result.current.status).toBe("active");
      });

      const activeState = result.current as Extract<PeriodStateResult, { status: "active" }>;
      expect(activeState.period).toEqual(testPeriod);
      expect(activeState.period.id).toBe("period-abc");
      expect(activeState.retry).toBeInstanceOf(Function);
    });

    it("period is guaranteed non-null when status is active", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("active");
      });

      if (result.current.status === "active") {
        // TypeScript enforces: period is BudgetPeriod, not BudgetPeriod | null
        const periodId: string = result.current.period.id;
        expect(periodId).toBe("period-abc");
      }
    });
  });

  describe("loading → no-period on 404", () => {
    it("transitions to no-period with defaults when PERIOD_NOT_FOUND", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      expect(result.current.status).toBe("loading");

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      const noPeriodState = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
      expect(noPeriodState.defaults).toEqual(testDefaults);
      expect(noPeriodState.createPeriod).toBeInstanceOf(Function);
      expect(noPeriodState.creating).toBe(false);
      expect(noPeriodState.createError).toBeNull();
      expect(noPeriodState.clearCreateError).toBeInstanceOf(Function);
      expect(noPeriodState.retry).toBeInstanceOf(Function);
    });

    it("transitions to no-period with null defaults when defaults fetch fails", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Database error" },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      const noPeriodState = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
      expect(noPeriodState.defaults).toBeNull();
    });
  });

  describe("loading → error on failure", () => {
    it("transitions to error on non-404 server error", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Database connection failed" },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      expect(result.current.status).toBe("loading");

      await waitFor(() => {
        expect(result.current.status).toBe("error");
      });

      expect(result.current.retry).toBeInstanceOf(Function);
    });

    it("retry transitions back to loading then to active on success", async () => {
      // First: error, then retry with success
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Temporary failure" },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("error");
      });

      // Swap to successful mock
      global.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
      }) as unknown as typeof fetch;

      act(() => {
        result.current.retry();
      });

      await waitFor(() => {
        expect(result.current.status).toBe("active");
      });

      const activeState = result.current as Extract<PeriodStateResult, { status: "active" }>;
      expect(activeState.period.id).toBe("period-abc");
    });
  });

  describe("no-period → active after createPeriod success", () => {
    it("transitions to active after successful period creation", async () => {
      const createdPeriod = buildPeriod({
        id: "new-period-123",
        userId: "user-1",
        year: 2026,
        month: 5,
        budgetAmount: 300000,
      });

      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": {
          status: 201,
          body: { period: createdPeriod, appliedProRata: [] },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      const noPeriodState = result.current as Extract<PeriodStateResult, { status: "no-period" }>;

      act(() => {
        noPeriodState.createPeriod({
          year: 2026,
          month: 5,
          budgetAmount: 300000,
          essentialsPercent: 50,
          desiresPercent: 30,
          savingsPercent: 20,
          reportingCurrencyCode: "USD",
        });
      });

      await waitFor(() => {
        expect(result.current.status).toBe("active");
      });

      const activeState = result.current as Extract<PeriodStateResult, { status: "active" }>;
      expect(activeState.period.id).toBe("new-period-123");
      expect(activeState.period.budgetAmount).toBe(300000);
    });

    it("shows creating state during period creation", async () => {
      let resolveCreate: (value: unknown) => void;
      const createPromise = new Promise((resolve) => {
        resolveCreate = resolve;
      });

      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      // Override fetch to hang on create
      const originalFetch = global.fetch;
      global.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === "string" ? input : input.toString();
        if (url.includes("/api/finance/periods") && !url.includes("/current")) {
          return createPromise.then((body) =>
            new Response(JSON.stringify(body), { status: 201, headers: { "content-type": "application/json" } }),
          );
        }
        return originalFetch(input, init);
      }) as typeof fetch;

      const noPeriodState = result.current as Extract<PeriodStateResult, { status: "no-period" }>;

      act(() => {
        noPeriodState.createPeriod({
          year: 2026,
          month: 5,
          budgetAmount: 300000,
          essentialsPercent: 50,
          desiresPercent: 30,
          savingsPercent: 20,
          reportingCurrencyCode: "USD",
        });
      });

      await waitFor(() => {
        const current = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
        expect(current.creating).toBe(true);
      });

      // Resolve the create
      const createdPeriod = buildPeriod({ id: "new-period-456" });
      await act(async () => {
        resolveCreate!({ period: createdPeriod, appliedProRata: [] });
      });

      await waitFor(() => {
        expect(result.current.status).toBe("active");
      });
    });

    it("stays in no-period with createError on failed creation", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": {
          status: 400,
          body: { code: "VALIDATION_ERROR", message: "Budget amount must be positive" },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      const noPeriodState = result.current as Extract<PeriodStateResult, { status: "no-period" }>;

      act(() => {
        noPeriodState.createPeriod({
          year: 2026,
          month: 5,
          budgetAmount: -100,
          essentialsPercent: 50,
          desiresPercent: 30,
          savingsPercent: 20,
          reportingCurrencyCode: "USD",
        });
      });

      await waitFor(() => {
        const current = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
        expect(current.createError).not.toBeNull();
      });

      const errorState = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
      expect(errorState.status).toBe("no-period");
      expect(errorState.createError).toBe("Budget amount must be positive");
    });

    it("clearCreateError resets the error to null", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
        "/api/finance/periods": {
          status: 400,
          body: { code: "VALIDATION_ERROR", message: "Invalid data" },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      act(() => {
        (result.current as Extract<PeriodStateResult, { status: "no-period" }>).createPeriod({
          year: 2026,
          month: 5,
          budgetAmount: 300000,
          essentialsPercent: 50,
          desiresPercent: 30,
          savingsPercent: 20,
          reportingCurrencyCode: "USD",
        });
      });

      await waitFor(() => {
        const current = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
        expect(current.createError).not.toBeNull();
      });

      act(() => {
        (result.current as Extract<PeriodStateResult, { status: "no-period" }>).clearCreateError();
      });

      const current = result.current as Extract<PeriodStateResult, { status: "no-period" }>;
      expect(current.createError).toBeNull();
    });
  });

  describe("variant-specific property access", () => {
    it("does not expose period on loading variant", async () => {
      global.fetch = (() => new Promise(() => {})) as unknown as typeof fetch;
      const { result } = renderHook(() => usePeriodState());

      expect(result.current.status).toBe("loading");
      expect("period" in result.current).toBe(false);
      expect("defaults" in result.current).toBe(false);
      expect("createPeriod" in result.current).toBe(false);
    });

    it("does not expose period on no-period variant", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 404,
          body: { code: "PERIOD_NOT_FOUND", message: "No budget period found" },
        },
        "/api/finance/defaults": { body: { defaults: testDefaults } },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("no-period");
      });

      expect("period" in result.current).toBe(false);
    });

    it("does not expose defaults or createPeriod on active variant", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": { body: { period: testPeriod } },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("active");
      });

      expect("defaults" in result.current).toBe(false);
      expect("createPeriod" in result.current).toBe(false);
      expect("creating" in result.current).toBe(false);
    });

    it("does not expose period or defaults on error variant", async () => {
      global.fetch = createMockApi({
        "/api/finance/periods/current": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Failure" },
        },
      }) as unknown as typeof fetch;

      const { result } = renderHook(() => usePeriodState());

      await waitFor(() => {
        expect(result.current.status).toBe("error");
      });

      expect("period" in result.current).toBe(false);
      expect("defaults" in result.current).toBe(false);
      expect("createPeriod" in result.current).toBe(false);
    });
  });
});
