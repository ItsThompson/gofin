import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { NewExpenseFeature } from "../index";
import type { User } from "@gofin/core";

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
  { id: "tag-bills", name: "Bills", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
];

function mockTagsResponse() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ tags: mockTags }),
  });
}

function renderNewExpense() {
  mockTagsResponse();
  return render(
    <MemoryRouter>
      <NewExpenseFeature user={mockUser} />
    </MemoryRouter>,
  );
}

describe("NewExpenseFeature - Pro-rata flow", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
  });

  it("shows pro-rata months field when checkbox is toggled", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    const checkbox = screen.getByLabelText("Spread across months");
    await user.click(checkbox);

    expect(screen.getByLabelText("Number of months")).toBeInTheDocument();
  });

  it("hides pro-rata months field when checkbox is unchecked", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    const checkbox = screen.getByLabelText("Spread across months");
    await user.click(checkbox);
    expect(screen.getByLabelText("Number of months")).toBeInTheDocument();

    await user.click(checkbox);
    expect(screen.queryByLabelText("Number of months")).not.toBeInTheDocument();
  });

  it("validates pro-rata months must be at least 2", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    await user.type(screen.getByLabelText("Name"), "Annual subscription");
    await user.type(screen.getByLabelText("Amount"), "120.00");

    const checkbox = screen.getByLabelText("Spread across months");
    await user.click(checkbox);

    // Leave months empty (don't type anything) to trigger validation
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(screen.getByText("Must be at least 2 months")).toBeInTheDocument();
    });
  });

  it("submits pro-rata expense to /api/finance/prorata endpoint", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: () =>
        Promise.resolve({
          schedule: {
            id: "prorata-1",
            name: "Annual subscription",
            totalAmount: 12000,
            months: 3,
          },
        }),
    });

    await user.type(screen.getByLabelText("Name"), "Annual subscription");
    await user.type(screen.getByLabelText("Amount"), "120.00");
    await user.click(screen.getByLabelText("savings"));

    const checkbox = screen.getByLabelText("Spread across months");
    await user.click(checkbox);

    const monthsInput = screen.getByLabelText("Number of months");
    await user.type(monthsInput, "3");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/dashboard");
    });

    const proRataCall = mockFetch.mock.calls.find(
      (call) =>
        typeof call[0] === "string" &&
        call[0].includes("/api/finance/prorata") &&
        call[1]?.method === "POST",
    );
    expect(proRataCall).toBeDefined();
    const body = JSON.parse(proRataCall![1].body);
    expect(body.totalAmount).toBe(12000);
    expect(body.months).toBe(3);
    expect(body.name).toBe("Annual subscription");
    expect(body.expenseType).toBe("savings");
  });

  it("shows network error message when network error occurs", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    mockFetch.mockRejectedValueOnce(new TypeError("Network error"));

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(
        screen.getByText("Connection lost. Check your internet and try again."),
      ).toBeInTheDocument();
    });
  });

  it("clears pro-rata months when checkbox is unchecked", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    const checkbox = screen.getByLabelText("Spread across months");
    await user.click(checkbox);

    const monthsInput = screen.getByLabelText("Number of months");
    await user.type(monthsInput, "6");

    // Uncheck: should clear the months value
    await user.click(checkbox);

    // Re-check: months should be empty
    await user.click(checkbox);
    const monthsInputAgain = screen.getByLabelText("Number of months") as HTMLInputElement;
    expect(monthsInputAgain.value).toBe("");
  });

  it("clears field errors when input changes", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    // Submit empty to trigger errors
    await user.click(screen.getByRole("button", { name: "Log Expense" }));
    expect(screen.getByText("Name is required")).toBeInTheDocument();

    // Typing in name should clear the name error
    await user.type(screen.getByLabelText("Name"), "Coffee");
    expect(screen.queryByText("Name is required")).not.toBeInTheDocument();
  });

  it("validates empty date field", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => {
      expect(screen.getByLabelText("Tag")).not.toHaveTextContent("Loading tags...");
    });

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    // Clear the date field
    const dateInput = screen.getByLabelText("Date");
    await user.clear(dateInput);

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    expect(screen.getByText("Date is required")).toBeInTheDocument();
  });
});
