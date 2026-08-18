import { describe, it, expect } from "vitest";
import { buildDefaults, buildUser } from "@gofin/test-utils";
import { initialReportingCurrency } from "../utils/initialReportingCurrency";

describe("initialReportingCurrency", () => {
  it("uses the defaults currency when supported, normalized to upper case", () => {
    const defaults = buildDefaults({ currency: "jpy" });
    expect(initialReportingCurrency(defaults, buildUser())).toBe("JPY");
  });

  it("falls back to the user currency when defaults are null", () => {
    expect(initialReportingCurrency(null, buildUser({ currency: "eur" }))).toBe(
      "EUR",
    );
  });

  it("falls back to the user currency when the defaults currency is unsupported", () => {
    const defaults = buildDefaults({ currency: "ZZZ" });
    expect(initialReportingCurrency(defaults, buildUser({ currency: "gbp" }))).toBe(
      "GBP",
    );
  });

  it("returns an empty string when no supported candidate exists", () => {
    const defaults = buildDefaults({ currency: "ZZZ" });
    expect(initialReportingCurrency(defaults, buildUser({ currency: "ZZZ" }))).toBe(
      "",
    );
    expect(initialReportingCurrency(null, buildUser({ currency: "ZZZ" }))).toBe(
      "",
    );
    expect(initialReportingCurrency(null, buildUser({ currency: "" }))).toBe("");
  });

  it("trims whitespace from the candidate", () => {
    expect(initialReportingCurrency(null, buildUser({ currency: " usd " }))).toBe(
      "USD",
    );
  });
});
