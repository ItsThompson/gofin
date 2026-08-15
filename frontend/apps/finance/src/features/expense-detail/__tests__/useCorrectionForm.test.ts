import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useCorrectionForm } from "../hooks/useCorrectionForm";
import type { Expense, Tag } from "@gofin/core";
import type { ExpenseSuggestion } from "../../expense-autocomplete";

const mockTags: Tag[] = [
  {
    id: "tag-food",
    name: "Food",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "tag-transport",
    name: "Transport",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

const mockSuggestion: ExpenseSuggestion = {
  name: "Train Pass",
  transactionAmount: 1234,
  transactionCurrency: "USD",
  expenseType: "desires",
  tagId: "tag-transport",
  frequency: 4,
  lastUsedAt: "2026-05-28T19:02:11Z",
  recencyBucket: "last_7_days",
  frecencyScore: 22,
};

const mockExpense: Expense = {
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

function createFormEvent(): React.FormEvent {
  return { preventDefault: vi.fn() } as unknown as React.FormEvent;
}

describe("useCorrectionForm", () => {
  describe("initial state", () => {
    it("pre-fills fields from the expense", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      expect(result.current.state.fields).toEqual({
        name: "Groceries",
        amountDollars: "50.00",
        expenseType: "essentials",
        tagId: "tag-food",
        expenseDate: "2026-05-02",
      });
    });

    it("starts with empty field errors", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      expect(result.current.state.fieldErrors).toEqual({});
    });

    it("derives amountDollars correctly from cents", () => {
      const expenseWithOddAmount: Expense = {
        ...mockExpense,
        transactionAmount: 1299, // $12.99
      };
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(expenseWithOddAmount, onSubmit),
      );

      expect(result.current.state.fields.amountDollars).toBe("12.99");
    });
  });

  describe("setField", () => {
    it("updates a field value", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("name", "Updated Groceries");
      });

      expect(result.current.state.fields.name).toBe("Updated Groceries");
    });

    it("clears the corresponding field error when set", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      // Trigger validation to produce errors: clear name first
      act(() => {
        result.current.actions.setField("name", "");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(result.current.state.fieldErrors.name).toBe("Name is required");

      // Setting the field should clear the error
      act(() => {
        result.current.actions.setField("name", "Fixed");
      });

      expect(result.current.state.fieldErrors.name).toBeUndefined();
    });
  });

  describe("clearFieldError", () => {
    it("clears a specific field error", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      // Trigger validation error: clear tagId
      act(() => {
        result.current.actions.setField("tagId", "");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(result.current.state.fieldErrors.tagId).toBe("Tag is required");

      act(() => {
        result.current.actions.clearFieldError("tagId");
      });

      expect(result.current.state.fieldErrors.tagId).toBeUndefined();
    });
  });

  describe("applySuggestion", () => {
    it("updates name, amount, type, and valid tag from a selected suggestion", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit, mockTags),
      );

      act(() => {
        result.current.actions.applySuggestion(mockSuggestion);
      });

      expect(result.current.state.fields).toEqual({
        name: "Train Pass",
        amountDollars: "12.34",
        expenseType: "desires",
        tagId: "tag-transport",
        expenseDate: "2026-05-02",
      });
    });

    it("keeps the current tag when the selected suggestion tag is stale", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit, mockTags),
      );

      act(() => {
        result.current.actions.applySuggestion({
          ...mockSuggestion,
          tagId: "deleted-tag",
        });
      });

      expect(result.current.state.fields.tagId).toBe("tag-food");
      expect(result.current.state.fields.name).toBe("Train Pass");
      expect(result.current.state.fields.amountDollars).toBe("12.34");
      expect(result.current.state.fields.expenseType).toBe("desires");
    });
  });

  describe("handleSubmit", () => {
    it("prevents default form event", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      const event = createFormEvent();
      act(() => {
        result.current.actions.handleSubmit(event);
      });

      expect(event.preventDefault).toHaveBeenCalledTimes(1);
    });

    it("calls onSubmit with CorrectExpenseRequest when valid", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).toHaveBeenCalledWith({
        name: "Groceries",
        amount: 5000,
        transactionCurrency: "USD",
        expenseType: "essentials",
        tagId: "tag-food",
        expenseDate: "2026-05-02",
      });
    });

    it("trims the name before submitting", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("name", "  Padded Name  ");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Padded Name" }),
      );
    });

    it("does not call onSubmit when validation fails", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      // Clear required name field
      act(() => {
        result.current.actions.setField("name", "");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).not.toHaveBeenCalled();
    });

    it("sets fieldErrors when validation fails", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      // Clear name and tagId
      act(() => {
        result.current.actions.setField("name", "");
      });
      act(() => {
        result.current.actions.setField("tagId", "");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(result.current.state.fieldErrors.name).toBe("Name is required");
      expect(result.current.state.fieldErrors.tagId).toBe("Tag is required");
    });

    it("does not validate pro-rata fields", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      // Submit with valid fields: should succeed without pro-rata validation
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).toHaveBeenCalled();
      expect(result.current.state.fieldErrors.proRataMonths).toBeUndefined();
    });

    it("converts amountDollars to cents correctly", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("amountDollars", "12.99");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ amount: 1299 }),
      );
    });

    it("validates amount must be greater than 0", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("amountDollars", "0");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).not.toHaveBeenCalled();
      expect(result.current.state.fieldErrors.amount).toBe(
        "Amount must be greater than 0",
      );
    });

    it("validates date is required", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("expenseDate", "");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).not.toHaveBeenCalled();
      expect(result.current.state.fieldErrors.expenseDate).toBe(
        "Date is required",
      );
    });

    it("submits updated field values after changes", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("name", "Updated Name");
      });
      act(() => {
        result.current.actions.setField("amountDollars", "75.50");
      });
      act(() => {
        result.current.actions.setField("expenseType", "desires");
      });
      act(() => {
        result.current.actions.setField("tagId", "tag-transport");
      });
      act(() => {
        result.current.actions.setField("expenseDate", "2026-05-10");
      });
      act(() => {
        result.current.actions.handleSubmit(createFormEvent());
      });

      expect(onSubmit).toHaveBeenCalledWith({
        name: "Updated Name",
        amount: 7550,
        transactionCurrency: "USD",
        expenseType: "desires",
        tagId: "tag-transport",
        expenseDate: "2026-05-10",
      });
    });

    it("revalidates amount precision when transaction currency changes", () => {
      const onSubmit = vi.fn();
      const { result } = renderHook(() =>
        useCorrectionForm(mockExpense, onSubmit),
      );

      act(() => {
        result.current.actions.setField("amountDollars", "50.50");
      });
      act(() => {
        result.current.actions.setTransactionCurrency("JPY");
      });

      expect(result.current.state.transactionCurrency).toBe("JPY");
      expect(result.current.state.fieldErrors.amount).toBe("Amount must be a whole JPY amount");
    });
  });
});
