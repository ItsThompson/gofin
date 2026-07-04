import { vi, expect } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { User } from "@gofin/core";

import { NewExpenseFeature } from "../index";
import type { ExpenseSuggestionsResponse } from "../../expense-autocomplete";
import type { Tag } from "../../../types";

// Shared fetch mock. Each test file resets this in beforeEach and layers its
// own mockResolvedValueOnce/mockRejectedValueOnce/mockImplementation as needed.
export const mockFetch = vi.fn();
global.fetch = mockFetch;

export const mockUser: User = {
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

export const mockTags: Tag[] = [
  {
    id: "tag-bills",
    name: "Bills",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "tag-food",
    name: "Food",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

export const mockSuggestions: ExpenseSuggestionsResponse = {
  data: [
    {
      name: "Coffee Shop",
      amount: 450,
      currency: "USD",
      expenseType: "desires",
      tagId: "tag-food",
      frequency: 4,
      lastUsedAt: "2026-05-25T00:00:00Z",
      recencyBucket: "last_7_days",
      frecencyScore: 42,
    },
    {
      name: "Coffee Beans",
      amount: 1200,
      currency: "USD",
      expenseType: "essentials",
      tagId: "tag-food",
      frequency: 2,
      lastUsedAt: "2026-05-20T00:00:00Z",
      recencyBucket: "last_30_days",
      frecencyScore: 31,
    },
  ],
  total: 2,
  page: 1,
  pageSize: 50,
  hasMore: false,
};

const emptySuggestions: ExpenseSuggestionsResponse = {
  data: [],
  total: 0,
  page: 1,
  pageSize: 50,
  hasMore: false,
};

export function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
}

type FetchResult = Promise<unknown>;
type FetchResponder = (url: string, init?: RequestInit) => FetchResult;
type ResponderOrResult = FetchResponder | FetchResult;

function toResponder(value: ResponderOrResult): FetchResponder {
  return typeof value === "function" ? value : () => value;
}

/** Per-endpoint overrides for {@link setNewExpenseFetchMock}. */
export interface NewExpenseFetchOverrides {
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
            amount: 450,
            currency: "USD",
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

export function countFetchCalls(path: string): number {
  return mockFetch.mock.calls.filter(
    (call) => typeof call[0] === "string" && call[0].includes(path),
  ).length;
}

export function findExpensePostCall() {
  return mockFetch.mock.calls.find(
    (call) =>
      typeof call[0] === "string" &&
      call[0].includes("/api/expenses") &&
      !call[0].includes("/api/expenses/suggestions") &&
      call[1]?.method === "POST",
  );
}

export function findProRataPostCall() {
  return mockFetch.mock.calls.find(
    (call) =>
      typeof call[0] === "string" &&
      call[0].includes("/api/finance/prorata") &&
      call[1]?.method === "POST",
  );
}

export function getSubmittedExpenseRequest() {
  const postCall = findExpensePostCall();
  return JSON.parse(postCall?.[1]?.body as string);
}

export async function waitForFormBootstrap() {
  await waitFor(() => {
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");
    expect(countFetchCalls("/api/expenses/suggestions")).toBe(1);
  });
}

/** Options for {@link renderNewExpense}. */
export interface RenderNewExpenseOptions {
  user?: User;
  tags?: Tag[];
  suggestions?: ExpenseSuggestionsResponse;
  expensePost?: ResponderOrResult;
  proRataPost?: ResponderOrResult;
  /** Full escape hatch: replace the fetch mock implementation entirely. */
  fetchHandler?: FetchResponder;
}

/**
 * Wires up the shared fetch mock and renders the new-expense feature inside a
 * router. Pass `suggestions`/`tags` to customize bootstrap data, per-endpoint
 * overrides for POST behavior, or a full `fetchHandler` for custom routing.
 */
export function renderNewExpense(options: RenderNewExpenseOptions = {}) {
  const { user = mockUser, tags, suggestions, expensePost, proRataPost, fetchHandler } =
    options;

  if (fetchHandler) {
    mockFetch.mockImplementation(fetchHandler);
  } else {
    setNewExpenseFetchMock({
      tags: tags ? jsonResponse({ tags }) : undefined,
      suggestions: suggestions ? jsonResponse(suggestions) : undefined,
      expensePost,
      proRataPost,
    });
  }

  return render(
    <MemoryRouter>
      <NewExpenseFeature user={user} />
    </MemoryRouter>,
  );
}
