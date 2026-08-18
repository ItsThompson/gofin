import { afterEach, describe, expect, it, vi } from "vitest";
import type { CurrencyCatalogFetcher } from "../src/currencyCatalog";
import { currencyCatalogFixture } from "./currency-fixtures";

type CurrencyCatalogModule = typeof import("../src/currencyCatalog");

/**
 * The catalog module holds module-level cache state, so each test group gets a
 * fresh module registry instead of sharing one across tests.
 */
async function freshCurrencyCatalog(): Promise<CurrencyCatalogModule> {
  vi.resetModules();
  return import("../src/currencyCatalog");
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("before the catalog loads", () => {
  it("exposes empty lists and fallback values", async () => {
    const catalog = await freshCurrencyCatalog();

    expect(catalog.SUPPORTED_CURRENCIES).toEqual([]);
    expect(catalog.SUPPORTED_CURRENCY_OPTIONS).toEqual([]);
    expect(catalog.isSupportedCurrency("USD")).toBe(false);
    expect(catalog.getCurrencySymbol("USD")).toBe("USD");
    expect(catalog.getMinorUnitDigits("USD")).toBe(2);
  });
});

describe("loadSupportedCurrencies", () => {
  it("fills the cache from the fetcher result", async () => {
    const catalog = await freshCurrencyCatalog();

    const loaded = await catalog.loadSupportedCurrencies(
      async () => currencyCatalogFixture,
      [],
    );

    expect(loaded).toBe(true);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual(currencyCatalogFixture);
    expect(catalog.isSupportedCurrency("JPY")).toBe(true);
    expect(catalog.getCurrencySymbol("EUR")).toBe("€");
    expect(catalog.getMinorUnitDigits("JPY")).toBe(0);
  });

  it("derives option labels from symbols, using the code alone when the symbol equals the code", async () => {
    const catalog = await freshCurrencyCatalog();

    await catalog.loadSupportedCurrencies(
      async () => currencyCatalogFixture,
      [],
    );

    expect(catalog.SUPPORTED_CURRENCY_OPTIONS).toEqual([
      { code: "USD", label: "USD ($)" },
      { code: "EUR", label: "EUR (€)" },
      { code: "GBP", label: "GBP (£)" },
      { code: "JPY", label: "JPY (¥)" },
      { code: "CAD", label: "CAD (C$)" },
      { code: "AUD", label: "AUD (A$)" },
      { code: "CHF", label: "CHF" },
      { code: "CNY", label: "CNY (¥)" },
      { code: "SGD", label: "SGD (S$)" },
      { code: "HKD", label: "HKD (HK$)" },
    ]);
  });

  it("retries after a transient failure and succeeds on a later attempt", async () => {
    const catalog = await freshCurrencyCatalog();
    const fetcher = vi
      .fn<CurrencyCatalogFetcher>()
      .mockRejectedValueOnce(new Error("network down"))
      .mockResolvedValueOnce(currencyCatalogFixture);

    const loaded = await catalog.loadSupportedCurrencies(fetcher, [1]);

    expect(loaded).toBe(true);
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual(currencyCatalogFixture);
  });

  it("returns false and keeps the fallbacks when every attempt fails", async () => {
    const catalog = await freshCurrencyCatalog();
    const fetcher = vi
      .fn<CurrencyCatalogFetcher>()
      .mockRejectedValue(new Error("network down"));

    const loaded = await catalog.loadSupportedCurrencies(fetcher, []);

    expect(loaded).toBe(false);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual([]);
    expect(catalog.SUPPORTED_CURRENCY_OPTIONS).toEqual([]);
  });

  it("keeps the existing cache when a reload fails", async () => {
    const catalog = await freshCurrencyCatalog();
    await catalog.loadSupportedCurrencies(
      async () => currencyCatalogFixture,
      [],
    );

    const loaded = await catalog.loadSupportedCurrencies(
      async () => {
        throw new Error("network down");
      },
      [],
    );

    expect(loaded).toBe(false);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual(currencyCatalogFixture);
  });
});

describe("default API fetcher", () => {
  it("loads from GET /api/finance/currencies with credentials", async () => {
    const catalog = await freshCurrencyCatalog();
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ currencies: currencyCatalogFixture }),
    });
    vi.stubGlobal("fetch", fetchMock);

    const loaded = await catalog.loadSupportedCurrencies(undefined, []);

    expect(loaded).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith("/api/finance/currencies", {
      credentials: "include",
      headers: { Accept: "application/json" },
    });
    expect(catalog.SUPPORTED_CURRENCIES).toEqual(currencyCatalogFixture);
  });

  it("treats a non-ok response as a failed attempt", async () => {
    const catalog = await freshCurrencyCatalog();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: false, status: 401 }),
    );

    const loaded = await catalog.loadSupportedCurrencies(undefined, []);

    expect(loaded).toBe(false);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual([]);
  });

  it("treats a response without a currencies list as a failed attempt", async () => {
    const catalog = await freshCurrencyCatalog();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ unexpected: true }),
      }),
    );

    const loaded = await catalog.loadSupportedCurrencies(undefined, []);

    expect(loaded).toBe(false);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual([]);
  });
});

describe("subscriptions", () => {
  it("notifies listeners when the catalog is replaced and stops after unsubscribe", async () => {
    const catalog = await freshCurrencyCatalog();
    const listener = vi.fn();

    const unsubscribe = catalog.subscribeSupportedCurrencies(listener);
    await catalog.loadSupportedCurrencies(
      async () => currencyCatalogFixture,
      [],
    );
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    await catalog.loadSupportedCurrencies(
      async () => [currencyCatalogFixture[0]],
      [],
    );
    expect(listener).toHaveBeenCalledTimes(1);
    expect(catalog.SUPPORTED_CURRENCIES).toEqual([currencyCatalogFixture[0]]);
  });

  it("does not notify when a load fails", async () => {
    const catalog = await freshCurrencyCatalog();
    const listener = vi.fn();
    catalog.subscribeSupportedCurrencies(listener);

    await catalog.loadSupportedCurrencies(
      async () => {
        throw new Error("network down");
      },
      [],
    );

    expect(listener).not.toHaveBeenCalled();
  });
});
