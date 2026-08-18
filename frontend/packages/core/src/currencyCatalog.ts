/**
 * Frontend cache of the supported-currency catalog.
 *
 * The backend owns the catalog and serves it through GET /api/finance/currencies.
 * This module starts empty; loadSupportedCurrencies fills it from the API and
 * the app shell triggers the load once the user is authenticated. Until the
 * first successful load, lookups return fallback values and the exported lists
 * stay empty. There is deliberately no built-in catalog and no default
 * currency in the frontend.
 */

export interface SupportedCurrency {
  code: string;
  symbol: string;
  name: string;
  minorUnitDigits: number;
}

export type SupportedCurrencyCode = string;

export interface SupportedCurrencyOption {
  code: string;
  label: string;
}

/** Resolves to the catalog entries. Injectable so tests can skip the network. */
export type CurrencyCatalogFetcher = () => Promise<readonly SupportedCurrency[]>;

const CATALOG_ENDPOINT = "/api/finance/currencies";
const DEFAULT_MINOR_UNIT_DIGITS = 2;
const DEFAULT_RETRY_DELAYS_MS: readonly number[] = [1_000, 5_000];

interface CurrencyListResponse {
  currencies: SupportedCurrency[];
}

// Stable references, mutated in place on every successful load so direct
// readers and useSyncExternalStore subscribers always see current contents.
const cachedCurrencies: SupportedCurrency[] = [];
const cachedOptions: SupportedCurrencyOption[] = [];

export const SUPPORTED_CURRENCIES: readonly SupportedCurrency[] =
  cachedCurrencies;
export const SUPPORTED_CURRENCY_OPTIONS: readonly SupportedCurrencyOption[] =
  cachedOptions;

const listeners = new Set<() => void>();

function replaceCatalog(entries: readonly SupportedCurrency[]): void {
  cachedCurrencies.splice(0, cachedCurrencies.length, ...entries);
  cachedOptions.splice(
    0,
    cachedOptions.length,
    ...entries.map((currency) => ({
      code: currency.code,
      label:
        currency.symbol === currency.code
          ? currency.code
          : `${currency.code} (${currency.symbol})`,
    })),
  );
  for (const listener of listeners) {
    listener();
  }
}

function findCurrency(currencyCode: string): SupportedCurrency | undefined {
  return cachedCurrencies.find((currency) => currency.code === currencyCode);
}

export function isSupportedCurrency(currencyCode: string): boolean {
  return findCurrency(currencyCode) !== undefined;
}

export function getCurrencySymbol(currencyCode: string): string {
  return findCurrency(currencyCode)?.symbol ?? currencyCode;
}

export function getMinorUnitDigits(currencyCode: string): number {
  return (
    findCurrency(currencyCode)?.minorUnitDigits ?? DEFAULT_MINOR_UNIT_DIGITS
  );
}

/** Registers a listener for catalog replacements. Returns the unsubscribe function. */
export function subscribeSupportedCurrencies(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

async function fetchCatalogFromApi(): Promise<readonly SupportedCurrency[]> {
  const response = await fetch(CATALOG_ENDPOINT, {
    credentials: "include",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    throw new Error(`Currency catalog request failed: HTTP ${response.status}`);
  }
  const body = (await response.json()) as Partial<CurrencyListResponse>;
  if (!Array.isArray(body.currencies)) {
    throw new Error("Currency catalog response is missing its currencies list");
  }
  return body.currencies;
}

function wait(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

/**
 * Loads the catalog into the module cache, retrying transient failures.
 * Returns true when the load succeeds. Failed loads leave the cache as it
 * was, so consumers keep the empty-catalog fallbacks until a load succeeds.
 */
export async function loadSupportedCurrencies(
  fetchCatalog: CurrencyCatalogFetcher = fetchCatalogFromApi,
  retryDelaysMs: readonly number[] = DEFAULT_RETRY_DELAYS_MS,
): Promise<boolean> {
  for (let attempt = 0; attempt <= retryDelaysMs.length; attempt++) {
    if (attempt > 0) {
      await wait(retryDelaysMs[attempt - 1]);
    }
    try {
      replaceCatalog(await fetchCatalog());
      return true;
    } catch {
      // Transient failure: retry on the next attempt.
    }
  }
  return false;
}
