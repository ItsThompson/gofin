import { describe, expect, it } from "vitest";
import sourceCatalog from "../../../../shared/currency/catalog.json";
import { SUPPORTED_CURRENCIES } from "../src/constants";

describe("generated currency catalog", () => {
  it("matches the shared source code set and minor units", () => {
    const generatedByCode: ReadonlyMap<
      string,
      (typeof SUPPORTED_CURRENCIES)[number]
    > = new Map(
      SUPPORTED_CURRENCIES.map((currency) => [currency.code, currency]),
    );

    expect([...generatedByCode.keys()].sort()).toEqual(
      sourceCatalog.map((currency) => currency.code).sort(),
    );

    for (const sourceCurrency of sourceCatalog) {
      expect(generatedByCode.get(sourceCurrency.code)?.minorUnitDigits).toBe(
        sourceCurrency.minorUnitDigits,
      );
    }
  });
});
