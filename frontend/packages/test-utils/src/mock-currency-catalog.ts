import type { SupportedCurrency } from "@gofin/core";

/**
 * The supported-currency catalog as served by GET /api/finance/currencies.
 * Mirrors the backend catalog so tests exercise the real shape.
 */
export const mockCurrencyCatalog: readonly SupportedCurrency[] = [
  { code: "USD", symbol: "$", name: "US Dollar", minorUnitDigits: 2 },
  { code: "EUR", symbol: "€", name: "Euro", minorUnitDigits: 2 },
  { code: "GBP", symbol: "£", name: "British Pound", minorUnitDigits: 2 },
  { code: "JPY", symbol: "¥", name: "Japanese Yen", minorUnitDigits: 0 },
  { code: "CAD", symbol: "C$", name: "Canadian Dollar", minorUnitDigits: 2 },
  { code: "AUD", symbol: "A$", name: "Australian Dollar", minorUnitDigits: 2 },
  { code: "CHF", symbol: "CHF", name: "Swiss Franc", minorUnitDigits: 2 },
  { code: "CNY", symbol: "¥", name: "Chinese Yuan", minorUnitDigits: 2 },
  { code: "SGD", symbol: "S$", name: "Singapore Dollar", minorUnitDigits: 2 },
  { code: "HKD", symbol: "HK$", name: "Hong Kong Dollar", minorUnitDigits: 2 },
];
