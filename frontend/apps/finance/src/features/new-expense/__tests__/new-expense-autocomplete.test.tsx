import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import type { User } from "@gofin/core";

import { NewExpenseFeature } from "../index";
import type { ExpenseSuggestionsResponse } from "../types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockUser: User = {
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockTags = [
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

const mockSuggestions: ExpenseSuggestionsResponse = {
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

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
}

function renderNewExpense(suggestions: ExpenseSuggestionsResponse = mockSuggestions) {
  mockFetch.mockImplementation((url: string, init?: RequestInit) => {
    if (url.includes("/api/finance/tags")) {
      return jsonResponse({ tags: mockTags });
    }

    if (url.includes("/api/expenses/suggestions")) {
      return jsonResponse(suggestions);
    }

    if (url.includes("/api/expenses") && init?.method === "POST") {
      return jsonResponse({ expense: { id: "exp-1", name: "Custom Coffee" } }, 201);
    }

    return jsonResponse({ message: "Unhandled request" }, 404);
  });

  return render(
    <MemoryRouter>
      <NewExpenseFeature user={mockUser} />
    </MemoryRouter>,
  );
}

describe("NewExpenseFeature autocomplete integration", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
  });

  it("updates only the name field when typing in the combobox", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));

    await user.type(screen.getByLabelText("Amount"), "12.34");
    await user.click(screen.getByLabelText("desires"));
    await user.selectOptions(screen.getByLabelText("Tag"), "tag-food");
    await user.clear(screen.getByLabelText("Date"));
    await user.type(screen.getByLabelText("Date"), "2026-05-01");
    await user.click(screen.getByLabelText("Spread across months"));
    await user.type(screen.getByLabelText("Number of months"), "3");

    await user.type(screen.getByLabelText("Name"), "Coffee");

    expect(screen.getByLabelText("Name")).toHaveValue("Coffee");
    expect(screen.getByLabelText("Amount")).toHaveValue(12.34);
    expect(screen.getByLabelText("desires")).toBeChecked();
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-food");
    expect(screen.getByLabelText("Date")).toHaveValue("2026-05-01");
    expect(screen.getByLabelText("Spread across months")).toBeChecked();
    expect(screen.getByLabelText("Number of months")).toHaveValue(3);
  });

  it("renders loaded fuzzy suggestions with name and frecency score", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await user.type(screen.getByLabelText("Name"), "Coffee");

    expect(await screen.findByText("Coffee Shop")).toBeInTheDocument();
    expect(screen.getByText("Frecency score: 42")).toBeInTheDocument();
    expect(screen.getByText("Coffee Beans")).toBeInTheDocument();
    expect(screen.getByText("Frecency score: 31")).toBeInTheDocument();
  });

  it("shows no-match state without blocking manual submission or clearing input", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));

    await user.type(screen.getByLabelText("Name"), "Custom Coffee");
    expect(await screen.findByText("No matching expenses")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Custom Coffee");

    await user.type(screen.getByLabelText("Amount"), "5.00");
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith("/dashboard"));

    const postCall = mockFetch.mock.calls.find(
      (call) =>
        typeof call[0] === "string" &&
        call[0].includes("/api/expenses") &&
        !call[0].includes("/api/expenses/suggestions") &&
        call[1]?.method === "POST",
    );
    expect(JSON.parse(postCall?.[1]?.body as string).name).toBe("Custom Coffee");
  });

  it("keeps existing required-name validation for typed input", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    expect(screen.getByText("Name is required")).toBeInTheDocument();
  });
});
