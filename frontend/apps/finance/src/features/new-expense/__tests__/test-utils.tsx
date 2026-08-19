import { expect } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { User } from "@gofin/core";

import { NewExpenseFeature } from "../index";
import {
  jsonResponse,
  mockFetch,
  mockPeriod,
  mockUser,
  setNewExpenseFetchMock,
} from "../__mocks__";
import type { FetchResponder, ResponderOrResult } from "../__mocks__";
import type { ExpenseSuggestionsResponse } from "../../expense-autocomplete";
import type { Tag } from "@gofin/core";

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
    expect(countFetchCalls("/api/finance/periods/current")).toBe(1);
    expect(countFetchCalls("/api/expenses/suggestions")).toBe(1);
  });
}

/** Options for {@link renderNewExpense}. */
export interface RenderNewExpenseOptions {
  user?: User;
  period?: typeof mockPeriod;
  periodResponse?: ResponderOrResult;
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
  const { user = mockUser, period, periodResponse, tags, suggestions, expensePost, proRataPost, fetchHandler } =
    options;

  if (fetchHandler) {
    const respondPeriod = periodResponse ?? (period ? jsonResponse({ period }) : jsonResponse({ period: mockPeriod }));
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/finance/periods/current")) {
        return typeof respondPeriod === "function" ? respondPeriod(url, init) : respondPeriod;
      }
      return fetchHandler(url, init);
    });
  } else {
    setNewExpenseFetchMock({
      period: periodResponse ?? (period ? jsonResponse({ period }) : undefined),
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
