import { describe, it, expect } from "vitest";
import { validateEDSSplit } from "@gofin/core";

/**
 * Budget validation divergence tests.
 *
 * `validateEDSSplit` (shared, from `@gofin/core`) rejects percentages that are
 * negative OR that do not sum to 100%. These tests pin the non-negative rule so
 * a split like -10/60/50 (sums to 100) cannot pass validation.
 */
describe("validateEDSSplit", () => {
  describe("non-negative validation (present in DashboardPage, missing in SettingsPage)", () => {
    it("returns error when essentials is negative", () => {
      const result = validateEDSSplit(-10, 60, 50);
      expect(result).toBe("Percentages must be non-negative");
    });

    it("returns error when desires is negative", () => {
      const result = validateEDSSplit(60, -10, 50);
      expect(result).toBe("Percentages must be non-negative");
    });

    it("returns error when savings is negative", () => {
      const result = validateEDSSplit(60, 50, -10);
      expect(result).toBe("Percentages must be non-negative");
    });
  });

  describe("zero values (valid: zero is non-negative)", () => {
    it("returns null for 0/0/100 split", () => {
      const result = validateEDSSplit(0, 0, 100);
      expect(result).toBeNull();
    });

    it("returns null for 100/0/0 split", () => {
      const result = validateEDSSplit(100, 0, 0);
      expect(result).toBeNull();
    });

    it("returns null for 0/100/0 split", () => {
      const result = validateEDSSplit(0, 100, 0);
      expect(result).toBeNull();
    });
  });

  describe("sum validation", () => {
    it("returns null for valid 50/30/20 split", () => {
      const result = validateEDSSplit(50, 30, 20);
      expect(result).toBeNull();
    });

    it("returns error when sum is less than 100", () => {
      const result = validateEDSSplit(50, 30, 19);
      expect(result).toBe("Percentages must sum to 100% (currently 99%)");
    });

    it("returns error when sum exceeds 100", () => {
      const result = validateEDSSplit(50, 30, 21);
      expect(result).toBe("Percentages must sum to 100% (currently 101%)");
    });
  });
});
