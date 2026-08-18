import { beforeAll, describe, it, expect } from "vitest";
import {
  formatCurrency,
  getCurrencyInputStep,
  hasValidMinorUnitPrecision,
  toCents,
  toMajorUnits,
  toMinorUnits,
} from "../src/currency";
import {
  getCurrencySymbol,
  getMinorUnitDigits,
  loadSupportedCurrencies,
} from "../src/currencyCatalog";
import { currencyCatalogFixture } from "./currency-fixtures";

beforeAll(async () => {
  await loadSupportedCurrencies(async () => currencyCatalogFixture, []);
});

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

  it("returns ¥ for JPY", () => {
    expect(getCurrencySymbol("JPY")).toBe("¥");
  });

  it("returns multi-character symbols correctly", () => {
    expect(getCurrencySymbol("CAD")).toBe("C$");
    expect(getCurrencySymbol("HKD")).toBe("HK$");
    expect(getCurrencySymbol("SGD")).toBe("S$");
  });

  it("falls back to code for unknown currencies", () => {
    expect(getCurrencySymbol("XYZ")).toBe("XYZ");
    expect(getCurrencySymbol("BTC")).toBe("BTC");
  });
});

describe("formatCurrency", () => {
  describe("positive amounts", () => {
    it("formats cents to dollars with 2 decimal places", () => {
      expect(formatCurrency(300000, "USD")).toBe("$3,000.00");
    });

    it("formats small amounts", () => {
      expect(formatCurrency(99, "USD")).toBe("$0.99");
      expect(formatCurrency(1, "USD")).toBe("$0.01");
    });

    it("adds thousands separators", () => {
      expect(formatCurrency(12345678, "USD")).toBe("$123,456.78");
    });
  });

  describe("zero amount", () => {
    it("formats zero cents as $0.00", () => {
      expect(formatCurrency(0, "USD")).toBe("$0.00");
    });
  });

  describe("negative amounts", () => {
    it("prepends minus sign before symbol", () => {
      expect(formatCurrency(-50000, "USD")).toBe("-$500.00");
    });

    it("handles small negative amounts", () => {
      expect(formatCurrency(-1, "USD")).toBe("-$0.01");
    });
  });

  describe("different currencies", () => {
    it("uses correct currency symbol for EUR", () => {
      expect(formatCurrency(150000, "EUR")).toBe("€1,500.00");
    });

    it("uses correct currency symbol for GBP", () => {
      expect(formatCurrency(150000, "GBP")).toBe("£1,500.00");
    });

    it("uses zero decimal places for JPY", () => {
      expect(formatCurrency(100000, "JPY")).toBe("¥100,000");
    });

    it("falls back to code for unknown currency", () => {
      expect(formatCurrency(1000, "XYZ")).toBe("XYZ10.00");
    });
  });
});

describe("minor unit helpers", () => {
  it("reads currency precision from the currency catalog", () => {
    expect(getMinorUnitDigits("USD")).toBe(2);
    expect(getMinorUnitDigits("JPY")).toBe(0);
    expect(getMinorUnitDigits("XYZ")).toBe(2);
  });

  it("converts minor-unit amounts to major units", () => {
    expect(toMajorUnits(1234, "USD")).toBe(12.34);
    expect(toMajorUnits(1234, "JPY")).toBe(1234);
  });

  it("converts input strings by currency precision", () => {
    expect(toMinorUnits("10.50", "USD")).toBe(1050);
    expect(toMinorUnits("10", "JPY")).toBe(10);
  });

  it("builds number input steps from currency precision", () => {
    expect(getCurrencyInputStep("USD")).toBe("0.01");
    expect(getCurrencyInputStep("JPY")).toBe("1");
  });

  it("checks input precision against the currency minor unit", () => {
    expect(hasValidMinorUnitPrecision("10.50", "USD")).toBe(true);
    expect(hasValidMinorUnitPrecision("10.50", "JPY")).toBe(false);
    expect(hasValidMinorUnitPrecision("10", "JPY")).toBe(true);
  });
});

describe("toCents", () => {
  describe("valid dollar strings", () => {
    it("converts whole dollars to cents", () => {
      expect(toCents("10")).toBe(1000);
      expect(toCents("1")).toBe(100);
    });

    it("converts dollars with cents", () => {
      expect(toCents("10.50")).toBe(1050);
      expect(toCents("0.99")).toBe(99);
    });

    it("rounds to nearest cent for precision issues", () => {
      // 19.99 * 100 can produce floating-point errors without rounding
      expect(toCents("19.99")).toBe(1999);
      expect(toCents("0.1")).toBe(10);
      expect(toCents("0.01")).toBe(1);
    });

    it("handles zero", () => {
      expect(toCents("0")).toBe(0);
      expect(toCents("0.00")).toBe(0);
    });
  });

  describe("negative values", () => {
    it("converts negative dollar strings", () => {
      expect(toCents("-5.00")).toBe(-500);
      expect(toCents("-0.01")).toBe(-1);
    });
  });

  describe("invalid input", () => {
    it("returns 0 for non-numeric strings", () => {
      expect(toCents("abc")).toBe(0);
      expect(toCents("")).toBe(0);
    });

    it("returns 0 for whitespace", () => {
      expect(toCents("   ")).toBe(0);
    });
  });
});
