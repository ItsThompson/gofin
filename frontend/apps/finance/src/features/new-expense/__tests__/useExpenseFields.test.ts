import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useExpenseFields } from "../hooks/useExpenseFields";

describe("useExpenseFields", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-12T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("initial state", () => {
    it("returns default field values when no init provided", () => {
      const { result } = renderHook(() => useExpenseFields());

      expect(result.current.fields).toEqual({
        name: "",
        amountDollars: "",
        expenseType: "essentials",
        tagId: "",
        expenseDate: "2026-05-12",
      });
      expect(result.current.fieldErrors).toEqual({});
      expect(result.current.amountCents).toBe(0);
    });

    it("uses provided init values", () => {
      const { result } = renderHook(() =>
        useExpenseFields({
          name: "Coffee",
          amountDollars: "4.50",
          expenseType: "desires",
          tagId: "tag-1",
          expenseDate: "2026-03-15",
        }),
      );

      expect(result.current.fields).toEqual({
        name: "Coffee",
        amountDollars: "4.50",
        expenseType: "desires",
        tagId: "tag-1",
        expenseDate: "2026-03-15",
      });
    });

    it("uses today's date as default expenseDate", () => {
      const { result } = renderHook(() => useExpenseFields());
      expect(result.current.fields.expenseDate).toBe("2026-05-12");
    });
  });

  describe("setField", () => {
    it("updates a single field value", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.setField("name", "Groceries");
      });

      expect(result.current.fields.name).toBe("Groceries");
      expect(result.current.fields.amountDollars).toBe("");
    });

    it("clears corresponding field error when setting a field", () => {
      const { result } = renderHook(() => useExpenseFields());

      // Trigger validation to create errors
      act(() => {
        result.current.validate();
      });
      expect(result.current.fieldErrors.name).toBe("Name is required");

      // Setting the field clears its error
      act(() => {
        result.current.setField("name", "Coffee");
      });
      expect(result.current.fieldErrors.name).toBeUndefined();
    });

    it("clears amount error when setting amountDollars", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.validate();
      });
      expect(result.current.fieldErrors.amount).toBe("Amount must be greater than 0");

      act(() => {
        result.current.setField("amountDollars", "10.00");
      });
      expect(result.current.fieldErrors.amount).toBeUndefined();
    });

    it("does not modify other field errors", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.validate();
      });
      expect(result.current.fieldErrors.name).toBeDefined();
      expect(result.current.fieldErrors.amount).toBeDefined();

      act(() => {
        result.current.setField("name", "Coffee");
      });
      expect(result.current.fieldErrors.name).toBeUndefined();
      expect(result.current.fieldErrors.amount).toBe("Amount must be greater than 0");
    });
  });

  describe("clearFieldError", () => {
    it("clears a specific field error", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.validate();
      });
      expect(result.current.fieldErrors.name).toBeDefined();

      act(() => {
        result.current.clearFieldError("name");
      });
      expect(result.current.fieldErrors.name).toBeUndefined();
    });

    it("is a no-op for fields without errors", () => {
      const { result } = renderHook(() =>
        useExpenseFields({ name: "Coffee" }),
      );

      act(() => {
        result.current.validate();
      });

      const errorsBefore = { ...result.current.fieldErrors };
      act(() => {
        result.current.clearFieldError("name");
      });
      expect(result.current.fieldErrors).toEqual(errorsBefore);
    });
  });

  describe("validate", () => {
    it("returns true and sets no errors when all fields are valid", () => {
      const { result } = renderHook(() =>
        useExpenseFields({
          name: "Coffee",
          amountDollars: "4.50",
          expenseType: "essentials",
          tagId: "tag-1",
          expenseDate: "2026-05-12",
        }),
      );

      let isValid: boolean;
      act(() => {
        isValid = result.current.validate();
      });
      expect(isValid!).toBe(true);
      expect(result.current.fieldErrors).toEqual({});
    });

    it("returns false and sets errors for invalid fields", () => {
      const { result } = renderHook(() => useExpenseFields());

      let isValid: boolean;
      act(() => {
        isValid = result.current.validate();
      });
      expect(isValid!).toBe(false);
      expect(result.current.fieldErrors.name).toBe("Name is required");
      expect(result.current.fieldErrors.amount).toBe("Amount must be greater than 0");
      expect(result.current.fieldErrors.tagId).toBe("Tag is required");
    });

    it("validates pro-rata options when provided", () => {
      const { result } = renderHook(() =>
        useExpenseFields({
          name: "Annual sub",
          amountDollars: "120.00",
          tagId: "tag-1",
          expenseDate: "2026-05-12",
        }),
      );

      let isValid: boolean;
      act(() => {
        isValid = result.current.validate({ isProRata: true, proRataMonths: "1" });
      });
      expect(isValid!).toBe(false);
      expect(result.current.fieldErrors.proRataMonths).toBe("Must be at least 2 months");
    });

    it("passes pro-rata validation with valid months", () => {
      const { result } = renderHook(() =>
        useExpenseFields({
          name: "Annual sub",
          amountDollars: "120.00",
          tagId: "tag-1",
          expenseDate: "2026-05-12",
        }),
      );

      let isValid: boolean;
      act(() => {
        isValid = result.current.validate({ isProRata: true, proRataMonths: "3" });
      });
      expect(isValid!).toBe(true);
    });
  });

  describe("reset", () => {
    it("resets to default values when no init provided", () => {
      const { result } = renderHook(() =>
        useExpenseFields({ name: "Coffee", amountDollars: "5.00" }),
      );

      act(() => {
        result.current.setField("name", "Updated");
        result.current.validate(); // create errors
      });

      act(() => {
        result.current.reset();
      });

      expect(result.current.fields.name).toBe("");
      expect(result.current.fields.amountDollars).toBe("");
      expect(result.current.fields.expenseDate).toBe("2026-05-12");
      expect(result.current.fieldErrors).toEqual({});
    });

    it("resets to provided init values", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.setField("name", "Temp");
      });

      act(() => {
        result.current.reset({
          name: "Reset Name",
          amountDollars: "10.00",
          tagId: "tag-2",
        });
      });

      expect(result.current.fields.name).toBe("Reset Name");
      expect(result.current.fields.amountDollars).toBe("10.00");
      expect(result.current.fields.tagId).toBe("tag-2");
      expect(result.current.fields.expenseType).toBe("essentials");
    });

    it("clears all field errors on reset", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.validate();
      });
      expect(Object.keys(result.current.fieldErrors).length).toBeGreaterThan(0);

      act(() => {
        result.current.reset();
      });
      expect(result.current.fieldErrors).toEqual({});
    });
  });

  describe("amountCents", () => {
    it("derives 0 when amountDollars is empty", () => {
      const { result } = renderHook(() => useExpenseFields());
      expect(result.current.amountCents).toBe(0);
    });

    it("converts dollars to cents correctly", () => {
      const { result } = renderHook(() =>
        useExpenseFields({ amountDollars: "4.50" }),
      );
      expect(result.current.amountCents).toBe(450);
    });

    it("rounds to nearest cent", () => {
      const { result } = renderHook(() =>
        useExpenseFields({ amountDollars: "10.999" }),
      );
      expect(result.current.amountCents).toBe(1100);
    });

    it("updates when amountDollars changes", () => {
      const { result } = renderHook(() => useExpenseFields());

      act(() => {
        result.current.setField("amountDollars", "25.50");
      });
      expect(result.current.amountCents).toBe(2550);
    });

    it("returns 0 for non-numeric values", () => {
      const { result } = renderHook(() =>
        useExpenseFields({ amountDollars: "abc" }),
      );
      expect(result.current.amountCents).toBe(0);
    });
  });
});
