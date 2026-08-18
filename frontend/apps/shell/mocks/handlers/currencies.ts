import { http, HttpResponse } from "msw";
import type { SupportedCurrency } from "@gofin/core";
import { mockCurrencies } from "../data";
import { simulateLatency } from "./latency";

interface CurrencyListResponse {
  currencies: SupportedCurrency[];
}

export const currenciesHandlers = [
  http.get<never, never, CurrencyListResponse>(
    "/api/finance/currencies",
    async () => {
      await simulateLatency();
      return HttpResponse.json({ currencies: mockCurrencies });
    },
  ),
];
