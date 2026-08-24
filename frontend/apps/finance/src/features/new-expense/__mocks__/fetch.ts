import { vi } from "vitest";

import { mockPeriod, mockTags, emptySuggestions } from "./fixtures";

// Shared fetch mock. Each test file resets this in beforeEach and layers its
// own mockResolvedValueOnce/mockRejectedValueOnce/mockImplementation as needed.
export const mockFetch = vi.fn();
global.fetch = mockFetch;

export function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
}

type FetchResult = Promise<unknown>;
export type FetchResponder = (url: string, init?: RequestInit) => FetchResult;
export type ResponderOrResult = FetchResponder | FetchResult;

function toResponder(value: ResponderOrResult): FetchResponder {
  return typeof value === "function" ? value : () => value;
}

/** Per-endpoint overrides for {@link setNewExpenseFetchMock}. */
export interface NewExpenseFetchOverrides {
  period?: ResponderOrResult;
  tags?: ResponderOrResult;
  suggestions?: ResponderOrResult;
  expensePost?: ResponderOrResult;
  proRataPost?: ResponderOrResult;
  fallback?: ResponderOrResult;
}

/**
 * Assigns a default `mockFetch.mockImplementation` that handles the four
 * endpoints touched by the new-expense form (tags, suggestions, expense POST,
 * pro-rata POST). Each endpoint can be overridden with either a responder
 * function or a pre-built response promise. Unhandled requests resolve to 404.
 */
export function setNewExpenseFetchMock(
  overrides: NewExpenseFetchOverrides = {},
): void {
  const respondPeriod = toResponder(
    overrides.period ?? jsonResponse({ period: mockPeriod }),
  );
  const respondTags = toResponder(
    overrides.tags ?? jsonResponse({ tags: mockTags }),
  );
  const respondSuggestions = toResponder(
    overrides.suggestions ?? jsonResponse(emptySuggestions),
  );
  const respondExpensePost = toResponder(
    overrides.expensePost ??
      jsonResponse(
        {
          expense: {
            id: "exp-123",
            name: "Coffee",
            transactionAmount: 450,
            expenseType: "desires",
            status: "active",
          },
        },
        201,
      ),
  );
  const respondProRataPost = toResponder(
    overrides.proRataPost ?? jsonResponse({ schedule: { id: "prorata-1" } }, 201),
  );
  const respondFallback = toResponder(
    overrides.fallback ?? jsonResponse({ message: "Unhandled request" }, 404),
  );

  mockFetch.mockImplementation((url: string, init?: RequestInit) => {
    if (url.includes("/api/finance/periods/current")) {
      return respondPeriod(url, init);
    }
    if (url.includes("/api/finance/tags")) {
      return respondTags(url, init);
    }
    if (url.includes("/api/expenses/suggestions")) {
      return respondSuggestions(url, init);
    }
    if (url.includes("/api/finance/prorata") && init?.method === "POST") {
      return respondProRataPost(url, init);
    }
    if (url.includes("/api/expenses") && init?.method === "POST") {
      return respondExpensePost(url, init);
    }
    return respondFallback(url, init);
  });
}
