import { describe, it, expect } from "vitest";
import { formatCurrency, getCurrencySymbol } from "@gofin/core";

describe("getCurrencySymbol", () => {
  it("returns $ for USD", () => {
    expect(getCurrencySymbol("USD")).toBe("$");
  });

  it("returns € for EUR", () => {
    expect(getCurrencySymbol("EUR")).toBe("€");
  });

  it("returns £ for GBP", () => {
    expect(getCurrencySymbol("GBP")).toBe("£");
  });

  it("falls back to code for unknown currencies", () => {
    expect(getCurrencySymbol("XYZ")).toBe("XYZ");
  });
});

describe("formatCurrency", () => {
  it("formats cents to dollars with 2 decimal places", () => {
    expect(formatCurrency(300000, "USD")).toBe("$3,000.00");
  });

  it("formats zero cents", () => {
    expect(formatCurrency(0, "USD")).toBe("$0.00");
  });

  it("formats small amounts", () => {
    expect(formatCurrency(99, "USD")).toBe("$0.99");
  });

  it("uses correct currency symbol", () => {
    expect(formatCurrency(150000, "EUR")).toBe("€1,500.00");
    expect(formatCurrency(150000, "GBP")).toBe("£1,500.00");
  });

  it("handles negative amounts", () => {
    expect(formatCurrency(-50000, "USD")).toBe("-$500.00");
  });

  it("adds thousands separators", () => {
    expect(formatCurrency(12345678, "USD")).toBe("$123,456.78");
  });
});
