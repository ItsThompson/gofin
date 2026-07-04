import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { toast } from "sonner";
import { NewExpenseFeature } from "../index";
import type { User } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockToastSuccess = vi.mocked(toast.success);
const mockToastError = vi.mocked(toast.error);

// Mock useNavigate
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
  { id: "tag-bills", name: "Bills", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
];

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
}

let expensePostResponse: () => Promise<unknown>;

function setupFetchMocks() {
  expensePostResponse = () =>
    jsonResponse({
      expense: {
        id: "exp-123",
        name: "Coffee",
        amount: 450,
        currency: "USD",
        expenseType: "desires",
        status: "active",
      },
    }, 201);

  mockFetch.mockImplementation((url: string, init?: RequestInit) => {
    if (url.includes("/api/expenses/suggestions")) {
      return jsonResponse({
        data: [],
        total: 0,
        page: 1,
        pageSize: 50,
        hasMore: false,
      });
    }

    if (url.includes("/api/finance/tags")) {
      return jsonResponse({ tags: mockTags });
    }

    if (url.includes("/api/expenses") && init?.method === "POST") {
      return expensePostResponse();
    }

    return jsonResponse({ message: "Unhandled request" }, 404);
  });
}

function renderNewExpense(user: User = mockUser) {
  setupFetchMocks();
  return render(
    <MemoryRouter>
      <NewExpenseFeature user={user} />
    </MemoryRouter>,
  );
}

function getTodayDate(): string {
  const today = new Date();
  return `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;
}

function countFetchCalls(path: string): number {
  return mockFetch.mock.calls.filter(
    (call) => typeof call[0] === "string" && call[0].includes(path),
  ).length;
}

function findExpensePostCall() {
  return mockFetch.mock.calls.find(
    (call) =>
      typeof call[0] === "string" &&
      call[0].includes("/api/expenses") &&
      !call[0].includes("/api/expenses/suggestions") &&
      call[1]?.method === "POST",
  );
}

async function waitForFormBootstrap() {
  await waitFor(() => {
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");
    expect(countFetchCalls("/api/expenses/suggestions")).toBe(1);
  });
}

function expectFormResetToFreshDefaults() {
  expect(screen.getByLabelText("Name")).toHaveValue("");
  expect(screen.getByLabelText("Amount")).toHaveValue(null);
  expect(screen.getByLabelText("essentials")).toBeChecked();
  expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");
  expect(screen.getByLabelText("Date")).toHaveValue(getTodayDate());
}

describe("NewExpenseFeature", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
    mockToastSuccess.mockReset();
    mockToastError.mockReset();
  });

  it("renders the expense form with all fields", async () => {
    renderNewExpense();
    await waitForFormBootstrap();

    expect(screen.getByText("New Expense")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Amount")).toBeInTheDocument();
    expect(screen.getByLabelText("Date")).toBeInTheDocument();
    expect(screen.getByLabelText("Tag")).toBeInTheDocument();

    // Radio buttons for expense type
    expect(screen.getByLabelText("essentials")).toBeInTheDocument();
    expect(screen.getByLabelText("desires")).toBeInTheDocument();
    expect(screen.getByLabelText("savings")).toBeInTheDocument();

    // Submit button
    expect(
      screen.getByRole("button", { name: "Log Expense" }),
    ).toBeInTheDocument();
  });

  it("defaults date to today", async () => {
    renderNewExpense();
    await waitForFormBootstrap();

    expect(screen.getByLabelText("Date")).toHaveValue(getTodayDate());
  });

  it("defaults expense type to essentials", async () => {
    renderNewExpense();
    await waitForFormBootstrap();

    const essentialsRadio = screen.getByLabelText(
      "essentials",
    ) as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(true);
  });

  it("displays currency symbol from user profile", async () => {
    renderNewExpense({ ...mockUser, currency: "EUR" });
    await waitForFormBootstrap();

    expect(screen.getByText("€")).toBeInTheDocument();
  });

  it("shows validation errors for empty required fields", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    // Submit without filling any fields
    const submitButton = screen.getByRole("button", { name: "Log Expense" });
    await user.click(submitButton);

    expect(screen.getByText("Name is required")).toBeInTheDocument();
    expect(
      screen.getByText("Amount must be greater than 0"),
    ).toBeInTheDocument();
    expect(findExpensePostCall()).toBeUndefined();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("shows validation error when amount is not entered", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    const nameInput = screen.getByLabelText("Name");
    await user.type(nameInput, "Coffee");

    // Leave amount empty and submit
    const submitButton = screen.getByRole("button", { name: "Log Expense" });
    await user.click(submitButton);

    expect(
      screen.getByText("Amount must be greater than 0"),
    ).toBeInTheDocument();
    expect(findExpensePostCall()).toBeUndefined();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("converts dollar amount to cents and submits", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "4.50");
    await user.click(screen.getByLabelText("desires"));

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith("Expense saved");
    });
    expect(mockNavigate).not.toHaveBeenCalled();

    // Verify the POST request body
    const postCall = findExpensePostCall();
    expect(postCall).toBeDefined();
    const body = JSON.parse(postCall![1].body);
    expect(body.amount).toBe(450); // $4.50 = 450 cents
    expect(body.name).toBe("Coffee");
    expect(body.expenseType).toBe("desires");
    expect(body.currency).toBe("USD");
  });

  it("resets standard success to fresh defaults without refetching bootstrap data", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();
    const tagFetchCount = countFetchCalls("/api/finance/tags");
    const suggestionsFetchCount = countFetchCalls("/api/expenses/suggestions");

    expensePostResponse = () =>
      jsonResponse({
        expense: { id: "exp-123", name: "Groceries", status: "active" },
      }, 201);

    await user.type(screen.getByLabelText("Name"), "Groceries");
    await user.type(screen.getByLabelText("Amount"), "25.00");
    await user.click(screen.getByLabelText("desires"));
    await user.selectOptions(screen.getByLabelText("Tag"), "tag-food");
    await user.clear(screen.getByLabelText("Date"));
    await user.type(screen.getByLabelText("Date"), "2026-05-01");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith("Expense saved");
    });
    expect(mockNavigate).not.toHaveBeenCalled();

    await waitFor(() => {
      expectFormResetToFreshDefaults();
    });
    expect(countFetchCalls("/api/finance/tags")).toBe(tagFetchCount);
    expect(countFetchCalls("/api/expenses/suggestions")).toBe(suggestionsFetchCount);
  });

  it("shows API error message and generic failure toast on submission failure", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    expensePostResponse = () =>
      jsonResponse(
        {
          code: "VALIDATION_ERROR",
          message: "amount must be positive",
        },
        400,
      );

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(screen.getByText("amount must be positive")).toBeInTheDocument();
    });

    expect(mockToastError).toHaveBeenCalledWith("Failed to save expense");
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("disables only the submit button while submitting", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    // Never-resolving promise to keep the submitting state
    expensePostResponse = () => new Promise(() => {});

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    const submitButton = screen.getByRole("button", { name: "Log Expense" });
    await user.click(submitButton);

    expect(screen.getByRole("button", { name: "Saving..." })).toBeDisabled();
    expect(screen.getByLabelText("Name")).toBeEnabled();
    expect(screen.getByLabelText("Amount")).toBeEnabled();
    expect(screen.getByLabelText("Date")).toBeEnabled();
    expect(screen.getByLabelText("Tag")).toBeEnabled();
    expect(screen.getByLabelText("Spread across months")).toBeEnabled();
  });

  it("allows selecting different expense types", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    const savingsRadio = screen.getByLabelText("savings") as HTMLInputElement;
    await user.click(savingsRadio);
    expect(savingsRadio.checked).toBe(true);

    const essentialsRadio = screen.getByLabelText(
      "essentials",
    ) as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(false);
  });
});
