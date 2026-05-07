import { describe, it, expect } from "vitest";
import { validateEDSSplit } from "@/lib/validation";

/**
 * Budget validation divergence tests.
 *
 * The DashboardPage's CreatePeriodPrompt validates that E/D/S percentages are
 * both non-negative AND sum to 100%. The SettingsPage's inline `validateEDSSplit`
 * only checks the sum, allowing negative values to pass validation.
 *
 * This is a latent bug: a user could enter -10/60/50 in Settings (sums to 100)
 * and it would save successfully, but the same values would be rejected in the
 * Dashboard's period creation form.
 *
 * Ticket #15 will fix this divergence by having SettingsPage use the shared
 * `validateEDSSplit` from `@/lib/validation.ts` (which includes the non-negative
 * check).
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
