import { useSyncExternalStore } from "react";
import {
  SUPPORTED_CURRENCIES,
  SUPPORTED_CURRENCY_OPTIONS,
  subscribeSupportedCurrencies,
  type SupportedCurrency,
  type SupportedCurrencyOption,
} from "@gofin/core";

const EMPTY_CURRENCIES: readonly SupportedCurrency[] = [];
const EMPTY_OPTIONS: readonly SupportedCurrencyOption[] = [];

/**
 * The supported currencies, re-rendering when the catalog loads. Empty until
 * the bootstrap load resolves, matching the module's pre-load fallbacks.
 */
export function useSupportedCurrencies(): readonly SupportedCurrency[] {
  return useSyncExternalStore(
    subscribeSupportedCurrencies,
    () => SUPPORTED_CURRENCIES,
    () => EMPTY_CURRENCIES,
  );
}

/** Display-ready options for every supported currency, live-updated on load. */
export function useSupportedCurrencyOptions(): readonly SupportedCurrencyOption[] {
  return useSyncExternalStore(
    subscribeSupportedCurrencies,
    () => SUPPORTED_CURRENCY_OPTIONS,
    () => EMPTY_OPTIONS,
  );
}
