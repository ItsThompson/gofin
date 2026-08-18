import { describe, expect, it } from "vitest";
import type { SupportedCurrency } from "@gofin/core";
import { currenciesHandlers } from "../handlers/currencies";
import { resolveMockRequest } from "./drive";

describe("GET /api/finance/currencies", () => {
  it("returns the supported currency catalog", async () => {
    const res = await resolveMockRequest(
      currenciesHandlers,
      "/api/finance/currencies",
    );
    const { currencies } = (await res.json()) as {
      currencies: SupportedCurrency[];
    };

    expect(currencies.map((currency) => currency.code)).toEqual([
      "USD",
      "EUR",
      "GBP",
      "JPY",
      "CAD",
      "AUD",
      "CHF",
      "CNY",
      "SGD",
      "HKD",
    ]);
    expect(
      currencies.find((currency) => currency.code === "JPY")?.minorUnitDigits,
    ).toBe(0);
  });
});
