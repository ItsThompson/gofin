import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useBudgetSplitForm } from "../src/hooks/useBudgetSplitForm";

describe("useBudgetSplitForm", () => {
  describe("initial state", () => {
    it("uses DEFAULT_BUDGET_SPLIT when no options provided", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      expect(result.current.fields).toEqual({
        budgetDollars: "",
        essentials: "50",
        desires: "30",
        savings: "20",
      });
    });

    it("uses initialBudgetCents converted to dollars", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialBudgetCents: 150000 }),
      );

      expect(result.current.fields.budgetDollars).toBe("1500");
    });

    it("uses selected currency precision for initial budget display", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialBudgetCents: 150000, currency: "JPY" }),
      );

      expect(result.current.fields.budgetDollars).toBe("150000");
    });

    it("uses empty string for budgetDollars when initialBudgetCents is not provided", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialSplit: { essentials: 50, desires: 30, savings: 20 } }),
      );

      expect(result.current.fields.budgetDollars).toBe("");
    });

    it("uses '0' for budgetDollars when initialBudgetCents is 0", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialBudgetCents: 0 }),
      );

      expect(result.current.fields.budgetDollars).toBe("0");
    });

    it("uses initialSplit when provided", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({
          initialSplit: { essentials: 60, desires: 25, savings: 15 },
        }),
      );

      expect(result.current.fields.essentials).toBe("60");
      expect(result.current.fields.desires).toBe("25");
      expect(result.current.fields.savings).toBe("15");
    });

    it("uses both initialBudgetCents and initialSplit together", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({
          initialBudgetCents: 250000,
          initialSplit: { essentials: 40, desires: 40, savings: 20 },
        }),
      );

      expect(result.current.fields.budgetDollars).toBe("2500");
      expect(result.current.fields.essentials).toBe("40");
      expect(result.current.fields.desires).toBe("40");
      expect(result.current.fields.savings).toBe("20");
    });
  });

  describe("setField", () => {
    it("updates a single field value", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("budgetDollars", "2000");
      });

      expect(result.current.fields.budgetDollars).toBe("2000");
      expect(result.current.fields.essentials).toBe("50");
    });

    it("updates essentials and recomputes splitTotal", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "60");
      });

      expect(result.current.fields.essentials).toBe("60");
      expect(result.current.splitTotal).toBe(110); // 60 + 30 + 20
    });
  });

  describe("splitTotal (derived)", () => {
    it("computes sum of E+D+S percentages", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      expect(result.current.splitTotal).toBe(100); // 50 + 30 + 20
    });

    it("handles non-numeric field values as 0", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "abc");
      });

      expect(result.current.splitTotal).toBe(50); // 0 + 30 + 20
    });

    it("handles empty field values as 0", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "");
        result.current.setField("desires", "");
        result.current.setField("savings", "");
      });

      expect(result.current.splitTotal).toBe(0);
    });
  });

  describe("splitError (derived)", () => {
    it("returns null when percentages sum to 100 and are non-negative", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      expect(result.current.splitError).toBeNull();
    });

    it("returns error when percentages do not sum to 100", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "60");
      });

      expect(result.current.splitError).toBe(
        "Percentages must sum to 100% (currently 110%)",
      );
    });

    it("returns error for negative percentages", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({
          initialSplit: { essentials: -10, desires: 60, savings: 50 },
        }),
      );

      expect(result.current.splitError).toBe(
        "Percentages must be non-negative",
      );
    });

    it("updates reactively when fields change", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      expect(result.current.splitError).toBeNull();

      act(() => {
        result.current.setField("essentials", "80");
      });

      expect(result.current.splitError).not.toBeNull();

      act(() => {
        result.current.setField("desires", "10");
        result.current.setField("savings", "10");
      });

      expect(result.current.splitError).toBeNull();
    });
  });

  describe("validate", () => {
    it("returns null for valid split and non-negative budget", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialBudgetCents: 100000 }),
      );

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      expect(error!).toBeNull();
    });

    it("returns error when percentages do not sum to 100", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "60");
      });

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      expect(error!).toContain("100%");
    });

    it("returns error for negative percentages", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "-10");
        result.current.setField("desires", "60");
        result.current.setField("savings", "50");
      });

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      expect(error!).toBe("Percentages must be non-negative");
    });

    it("returns error for negative budget", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("budgetDollars", "-100");
      });

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      expect(error!).toBe("Budget amount must be non-negative");
    });

    it("returns null when budget is empty string (treated as 0)", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      expect(error!).toBeNull();
    });

    it("validates sum check first when both sum and non-negative fail", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "-10");
        result.current.setField("desires", "30");
        result.current.setField("savings", "20");
      });

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      // validateEDSSplit checks non-negative first
      expect(error!).toBe("Percentages must be non-negative");
    });

    it("checks split percentages allowing zeros that sum to 100", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "100");
        result.current.setField("desires", "0");
        result.current.setField("savings", "0");
      });

      let error: string | null;
      act(() => {
        error = result.current.validate();
      });

      expect(error!).toBeNull();
    });
  });

  describe("toPayload", () => {
    it("converts dollar string to cents using the selected currency", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("budgetDollars", "1500.50");
      });

      let payload: ReturnType<typeof result.current.toPayload>;
      act(() => {
        payload = result.current.toPayload();
      });

      expect(payload!.budgetAmountCents).toBe(150050);
    });

    it("returns 0 minor units for empty budget string", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      let payload: ReturnType<typeof result.current.toPayload>;
      act(() => {
        payload = result.current.toPayload();
      });

      expect(payload!.budgetAmountCents).toBe(0);
    });

    it("returns parsed integer percentages", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({
          initialSplit: { essentials: 60, desires: 25, savings: 15 },
        }),
      );

      let payload: ReturnType<typeof result.current.toPayload>;
      act(() => {
        payload = result.current.toPayload();
      });

      expect(payload!.essentialsPercent).toBe(60);
      expect(payload!.desiresPercent).toBe(25);
      expect(payload!.savingsPercent).toBe(15);
    });

    it("returns complete payload shape", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialBudgetCents: 200000 }),
      );

      let payload: ReturnType<typeof result.current.toPayload>;
      act(() => {
        payload = result.current.toPayload();
      });

      expect(payload!).toEqual({
        budgetAmountCents: 200000,
        essentialsPercent: 50,
        desiresPercent: 30,
        savingsPercent: 20,
      });
    });

    it("handles fractional cents with proper rounding", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("budgetDollars", "19.99");
      });

      let payload: ReturnType<typeof result.current.toPayload>;
      act(() => {
        payload = result.current.toPayload();
      });

      expect(payload!.budgetAmountCents).toBe(1999);
    });

    it("uses zero minor-unit digits for JPY payloads", () => {
      const { result } = renderHook(() => useBudgetSplitForm({ currency: "JPY" }));

      act(() => {
        result.current.setField("budgetDollars", "3000");
      });

      let payload: ReturnType<typeof result.current.toPayload>;
      act(() => {
        payload = result.current.toPayload();
      });

      expect(payload!.budgetAmountCents).toBe(3000);
    });
  });

  describe("reset", () => {
    it("resets to default values when called without arguments", () => {
      const { result } = renderHook(() =>
        useBudgetSplitForm({ initialBudgetCents: 100000 }),
      );

      act(() => {
        result.current.setField("budgetDollars", "9999");
        result.current.setField("essentials", "80");
      });

      act(() => {
        result.current.reset();
      });

      expect(result.current.fields).toEqual({
        budgetDollars: "",
        essentials: "50",
        desires: "30",
        savings: "20",
      });
    });

    it("resets to provided options", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.reset({
          initialBudgetCents: 300000,
          initialSplit: { essentials: 70, desires: 20, savings: 10 },
        });
      });

      expect(result.current.fields).toEqual({
        budgetDollars: "3000",
        essentials: "70",
        desires: "20",
        savings: "10",
      });
    });

    it("clears validation error after reset to valid values", () => {
      const { result } = renderHook(() => useBudgetSplitForm());

      act(() => {
        result.current.setField("essentials", "80");
      });

      expect(result.current.splitError).not.toBeNull();

      act(() => {
        result.current.reset();
      });

      expect(result.current.splitError).toBeNull();
    });
  });
});
