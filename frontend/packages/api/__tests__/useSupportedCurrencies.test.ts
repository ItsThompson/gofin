import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { loadSupportedCurrencies } from "@gofin/core";
import {
  useSupportedCurrencies,
  useSupportedCurrencyOptions,
} from "../src/hooks/useSupportedCurrencies";

const catalog = [
  { code: "USD", symbol: "$", name: "US Dollar", minorUnitDigits: 2 },
  { code: "JPY", symbol: "¥", name: "Japanese Yen", minorUnitDigits: 0 },
];

describe("currency catalog hooks", () => {
  // The catalog module is shared across this file's tests, so the empty-state
  // assertions must run before any test loads the catalog.
  it("returns empty lists before the catalog loads", () => {
    const currencies = renderHook(() => useSupportedCurrencies());
    const options = renderHook(() => useSupportedCurrencyOptions());

    expect(currencies.result.current).toEqual([]);
    expect(options.result.current).toEqual([]);
  });

  it("re-renders useSupportedCurrencies when the catalog loads", async () => {
    const { result } = renderHook(() => useSupportedCurrencies());

    await act(async () => {
      await loadSupportedCurrencies(async () => catalog, []);
    });

    expect(result.current).toEqual(catalog);
  });

  it("re-renders useSupportedCurrencyOptions with display-ready options", async () => {
    const { result } = renderHook(() => useSupportedCurrencyOptions());

    await act(async () => {
      await loadSupportedCurrencies(async () => catalog, []);
    });

    expect(result.current).toEqual([
      { code: "USD", label: "USD ($)" },
      { code: "JPY", label: "JPY (¥)" },
    ]);
  });
});
