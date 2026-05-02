import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { NewExpensePage } from "@/pages/NewExpensePage";
import type { User } from "@gofin/types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

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

function renderNewExpensePage(user: User = mockUser) {
  return render(
    <MemoryRouter>
      <NewExpensePage user={user} />
    </MemoryRouter>,
  );
}

describe("NewExpensePage", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
  });

  it("renders the expense form with all fields", () => {
    renderNewExpensePage();

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

  it("defaults date to today", () => {
    renderNewExpensePage();

    const dateInput = screen.getByLabelText("Date") as HTMLInputElement;
    const today = new Date();
    const expectedDate = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;
    expect(dateInput.value).toBe(expectedDate);
  });

  it("defaults expense type to essentials", () => {
    renderNewExpensePage();

    const essentialsRadio = screen.getByLabelText(
      "essentials",
    ) as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(true);
  });

  it("displays currency symbol from user profile", () => {
    renderNewExpensePage({ ...mockUser, currency: "EUR" });

    expect(screen.getByText("€")).toBeInTheDocument();
  });

  it("shows validation errors for empty required fields", async () => {
    const user = userEvent.setup();
    renderNewExpensePage();

    // Submit without filling any fields
    const submitButton = screen.getByRole("button", { name: "Log Expense" });
    await user.click(submitButton);

    expect(screen.getByText("Name is required")).toBeInTheDocument();
    expect(
      screen.getByText("Amount must be greater than 0"),
    ).toBeInTheDocument();
  });

  it("shows validation error when amount is not entered", async () => {
    const user = userEvent.setup();
    renderNewExpensePage();

    const nameInput = screen.getByLabelText("Name");
    await user.type(nameInput, "Coffee");

    // Leave amount empty and submit
    const submitButton = screen.getByRole("button", { name: "Log Expense" });
    await user.click(submitButton);

    expect(
      screen.getByText("Amount must be greater than 0"),
    ).toBeInTheDocument();
  });

  it("converts dollar amount to cents and submits", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: () =>
        Promise.resolve({
          expense: {
            id: "exp-123",
            name: "Coffee",
            amount: 450,
            currency: "USD",
            expenseType: "desires",
            status: "active",
          },
        }),
    });

    const user = userEvent.setup();
    renderNewExpensePage();

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "4.50");
    await user.click(screen.getByLabelText("desires"));

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/dashboard");
    });

    // Verify the POST request body
    const postCall = mockFetch.mock.calls.find(
      (call) =>
        typeof call[0] === "string" &&
        call[0].includes("/api/expenses") &&
        call[1]?.method === "POST",
    );
    expect(postCall).toBeDefined();
    const body = JSON.parse(postCall![1].body);
    expect(body.amount).toBe(450); // $4.50 = 450 cents
    expect(body.name).toBe("Coffee");
    expect(body.expenseType).toBe("desires");
    expect(body.currency).toBe("USD");
  });

  it("redirects to /dashboard on successful submission", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: () =>
        Promise.resolve({
          expense: { id: "exp-123", name: "Groceries", status: "active" },
        }),
    });

    const user = userEvent.setup();
    renderNewExpensePage();

    await user.type(screen.getByLabelText("Name"), "Groceries");
    await user.type(screen.getByLabelText("Amount"), "25.00");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/dashboard");
    });
  });

  it("shows API error message on submission failure", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: () =>
        Promise.resolve({
          code: "VALIDATION_ERROR",
          message: "amount must be positive",
        }),
    });

    const user = userEvent.setup();
    renderNewExpensePage();

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(screen.getByText("amount must be positive")).toBeInTheDocument();
    });

    // Should NOT navigate on error
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("disables submit button while submitting", async () => {
    // Never-resolving promise to keep the submitting state
    mockFetch.mockReturnValueOnce(new Promise(() => {}));

    const user = userEvent.setup();
    renderNewExpensePage();

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    const submitButton = screen.getByRole("button", { name: "Log Expense" });
    await user.click(submitButton);

    expect(screen.getByRole("button", { name: "Saving..." })).toBeDisabled();
  });

  it("allows selecting different expense types", async () => {
    const user = userEvent.setup();
    renderNewExpensePage();

    const savingsRadio = screen.getByLabelText("savings") as HTMLInputElement;
    await user.click(savingsRadio);
    expect(savingsRadio.checked).toBe(true);

    const essentialsRadio = screen.getByLabelText(
      "essentials",
    ) as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(false);
  });
});
