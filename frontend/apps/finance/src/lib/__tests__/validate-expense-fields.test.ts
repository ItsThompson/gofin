import { describe, it, expect } from "vitest";
import {
  validateExpenseFields,
  type ExpenseFields,
  type ValidateExpenseOptions,
} from "@/lib/validate-expense-fields";

function buildFields(overrides?: Partial<ExpenseFields>): ExpenseFields {
  return {
    name: "Groceries",
    amountDollars: "50.00",
    expenseType: "essentials",
    tagId: "tag-1",
    expenseDateIso: "2026-05-12",
    ...overrides,
  };
}

describe("validateExpenseFields", () => {
  it("returns empty record for a valid form", () => {
    const result = validateExpenseFields(buildFields());
    expect(result).toEqual({});
  });

  it("returns error when name is empty", () => {
    const result = validateExpenseFields(buildFields({ name: "" }));
    expect(result.name).toBe("Name is required");
  });

  it("returns error when name is whitespace-only", () => {
    const result = validateExpenseFields(buildFields({ name: "   " }));
    expect(result.name).toBe("Name is required");
  });

  it("returns error when amount is zero", () => {
    const result = validateExpenseFields(buildFields({ amountDollars: "0" }));
    expect(result.amount).toBe("Amount must be greater than 0");
  });

  it("returns error when amount is negative", () => {
    const result = validateExpenseFields(buildFields({ amountDollars: "-5" }));
    expect(result.amount).toBe("Amount must be greater than 0");
  });

  it("returns error when amount is NaN", () => {
    const result = validateExpenseFields(buildFields({ amountDollars: "abc" }));
    expect(result.amount).toBe("Amount must be greater than 0");
  });

  it("returns error when date is missing", () => {
    const result = validateExpenseFields(buildFields({ expenseDateIso: "" }));
    expect(result.expenseDateIso).toBe("Date is required");
  });

  it("returns error when tag is missing", () => {
    const result = validateExpenseFields(buildFields({ tagId: "" }));
    expect(result.tagId).toBe("Tag is required");
  });

  it("returns multiple errors simultaneously", () => {
    const result = validateExpenseFields(
      buildFields({ name: "", amountDollars: "0", tagId: "" }),
    );
    expect(Object.keys(result)).toHaveLength(3);
    expect(result.name).toBe("Name is required");
    expect(result.amount).toBe("Amount must be greater than 0");
    expect(result.tagId).toBe("Tag is required");
  });

  describe("pro-rata validation", () => {
    it("does not validate pro-rata months when isProRata is false", () => {
      const options: ValidateExpenseOptions = {
        isProRata: false,
        proRataMonths: "",
      };
      const result = validateExpenseFields(buildFields(), options);
      expect(result).toEqual({});
    });

    it("does not validate pro-rata months when options are undefined", () => {
      const result = validateExpenseFields(buildFields());
      expect(result.proRataMonths).toBeUndefined();
    });

    it("returns error when pro-rata months is less than 2", () => {
      const options: ValidateExpenseOptions = {
        isProRata: true,
        proRataMonths: "1",
      };
      const result = validateExpenseFields(buildFields(), options);
      expect(result.proRataMonths).toBe("Must be at least 2 months");
    });

    it("returns error when pro-rata months is empty", () => {
      const options: ValidateExpenseOptions = {
        isProRata: true,
        proRataMonths: "",
      };
      const result = validateExpenseFields(buildFields(), options);
      expect(result.proRataMonths).toBe("Must be at least 2 months");
    });

    it("returns error when pro-rata months is not a number", () => {
      const options: ValidateExpenseOptions = {
        isProRata: true,
        proRataMonths: "abc",
      };
      const result = validateExpenseFields(buildFields(), options);
      expect(result.proRataMonths).toBe("Must be at least 2 months");
    });

    it("passes when pro-rata months is 2 or more", () => {
      const options: ValidateExpenseOptions = {
        isProRata: true,
        proRataMonths: "2",
      };
      const result = validateExpenseFields(buildFields(), options);
      expect(result).toEqual({});
    });
  });
});
