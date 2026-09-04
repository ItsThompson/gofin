import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";

import { toLocalISODate } from "../../../lib/date-utils";
import {
  jsonResponse,
  mockFetch,
  mockPeriod,
  setNewExpenseFetchMock,
} from "../__mocks__";
import {
  countFetchCalls,
  findExpensePostCall,
  renderNewExpense,
  waitForFormBootstrap,
} from "./test-utils";

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

function expectFormResetToFreshDefaults() {
  expect(screen.getByLabelText("Name")).toHaveValue("");
  expect(screen.getByLabelText("Amount")).toHaveValue(null);
  expect(screen.getByLabelText("essentials")).toBeChecked();
  expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");
  expect(screen.getByLabelText("Date")).toHaveValue(toLocalISODate());
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
    expect(screen.getByLabelText("Transaction Currency")).toBeInTheDocument();

    expect(screen.getByLabelText("essentials")).toBeInTheDocument();
    expect(screen.getByLabelText("desires")).toBeInTheDocument();
    expect(screen.getByLabelText("savings")).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "Log Expense" }),
    ).toBeInTheDocument();
  });

  it("defaults date to today", async () => {
    renderNewExpense();
    await waitForFormBootstrap();

    expect(screen.getByLabelText("Date")).toHaveValue(toLocalISODate());
  });

  it("defaults expense type to essentials", async () => {
    renderNewExpense();
    await waitForFormBootstrap();

    const essentialsRadio = screen.getByLabelText(
      "essentials",
    ) as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(true);
  });

  it("displays currency symbol from the resolved budget period", async () => {
    renderNewExpense({ period: { ...mockPeriod, reportingCurrency: "EUR" } });
    await waitForFormBootstrap();

    expect(screen.getByText("€")).toBeInTheDocument();
  });

  it("shows validation errors for empty required fields", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

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

    await waitForFormBootstrap();

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
    expect(body.amount).toBe(450);
    expect(body.name).toBe("Coffee");
    expect(body.expenseType).toBe("desires");
    expect(body.transactionCurrency).toBe("USD");
    expect(body.currency).toBeUndefined();
    // The form generates an idempotency key per logical submit for dedup.
    expect(body.clientGeneratedIdempotencyKey).toEqual(expect.any(String));
    expect(body.clientGeneratedIdempotencyKey).toHaveLength(36);
  });

  it("rejects JPY decimals and keeps the field error on amount", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    await user.type(screen.getByLabelText("Name"), "Ramen");
    await user.selectOptions(screen.getByLabelText("Transaction Currency"), "JPY");
    fireEvent.change(screen.getByLabelText("Amount"), { target: { value: "1200.50" } });

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    expect(screen.getByText("Amount must be a whole JPY amount")).toBeInTheDocument();
    expect(screen.getByLabelText("Amount")).toHaveAttribute("aria-invalid", "true");
    expect(findExpensePostCall()).toBeUndefined();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("submits EUR decimal input as integer minor units", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    await user.type(screen.getByLabelText("Name"), "Museum");
    await user.selectOptions(screen.getByLabelText("Transaction Currency"), "EUR");
    await user.type(screen.getByLabelText("Amount"), "12.34");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith("Expense saved");
    });

    const postCall = findExpensePostCall();
    expect(postCall).toBeDefined();
    const body = JSON.parse(postCall![1].body);
    expect(body.amount).toBe(1234);
    expect(body.transactionCurrency).toBe("EUR");
    expect(body.currency).toBeUndefined();
  });

  it("shows a dashboard setup link when the selected period is missing", async () => {
    renderNewExpense({
      periodResponse: jsonResponse(
        {
          code: "PERIOD_NOT_FOUND",
          message: "No budget period found for 2026-05",
        },
        404,
      ),
    });

    await waitFor(() => {
      expect(screen.getByText("Create a budget period first")).toBeInTheDocument();
    });
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to dashboard setup" })).toHaveAttribute("href", "/dashboard");
  });

  it("shows the period load error when the period fetch fails unexpectedly", async () => {
    renderNewExpense({
      periodResponse: jsonResponse(
        { code: "INTERNAL_ERROR", message: "boom" },
        500,
      ),
    });

    await waitFor(() => {
      expect(screen.getByText("Create a budget period first")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load budget period context.")).toBeInTheDocument();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
  });

  it("resets standard success to fresh defaults without refetching bootstrap data", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();
    const tagFetchCount = countFetchCalls("/api/finance/tags");
    const suggestionsFetchCount = countFetchCalls("/api/expenses/suggestions");

    setNewExpenseFetchMock({
      expensePost: () =>
        jsonResponse({
          expense: { id: "exp-123", name: "Groceries", status: "active" },
        }, 201),
    });

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

    setNewExpenseFetchMock({
      expensePost: () =>
        jsonResponse(
          {
            code: "VALIDATION_ERROR",
            message: "amount must be positive",
          },
          400,
        ),
    });

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
    setNewExpenseFetchMock({ expensePost: () => new Promise(() => {}) });

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

    await waitForFormBootstrap();

    const savingsRadio = screen.getByLabelText("savings") as HTMLInputElement;
    await user.click(savingsRadio);
    expect(savingsRadio.checked).toBe(true);

    const essentialsRadio = screen.getByLabelText(
      "essentials",
    ) as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(false);
  });

  it("submits foreign-currency payload with transactionCurrency and shows conversion-unavailable banner on FX failure", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    setNewExpenseFetchMock({
      expensePost: () =>
        jsonResponse(
          {
            code: "CONVERSION_UNAVAILABLE",
            message:
              "Conversion unavailable. Try again later, or enter the manually converted amount in the period currency.",
          },
          503,
        ),
    });

    await user.type(screen.getByLabelText("Name"), "Cafe abroad");
    await user.selectOptions(
      screen.getByLabelText("Transaction Currency"),
      "EUR",
    );
    await user.type(screen.getByLabelText("Amount"), "12.50");
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    // The POST payload carries transactionCurrency (not legacy currency).
    const postCall = findExpensePostCall();
    expect(postCall).toBeDefined();
    const body = JSON.parse(postCall![1].body);
    expect(body.transactionCurrency).toBe("EUR");
    expect(body.currency).toBeUndefined();

    // The conversion-unavailable guidance banner is shown.
    await waitFor(() => {
      expect(
        screen.getByText(/Conversion unavailable/i),
      ).toBeInTheDocument();
    });

    // The toast shows the conversion-unavailable guidance, not the generic failure.
    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        expect.stringContaining("Conversion unavailable"),
      );
    });

    // Form values are preserved after the failed foreign-currency save.
    expect(screen.getByLabelText("Name")).toHaveValue("Cafe abroad");
    expect(screen.getByLabelText("Amount")).toHaveValue(12.5);
    expect(screen.getByLabelText("Transaction Currency")).toHaveValue("EUR");

    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("does not show a partially saved expense after FX failure (no refetch of expenses list)", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    setNewExpenseFetchMock({
      expensePost: () =>
        jsonResponse(
          {
            code: "CONVERSION_UNAVAILABLE",
            message:
              "Conversion unavailable. Try again later, or enter the manually converted amount in the period currency.",
          },
          503,
        ),
    });

    await user.type(screen.getByLabelText("Name"), "FX fail");
    await user.selectOptions(
      screen.getByLabelText("Transaction Currency"),
      "GBP",
    );
    await user.type(screen.getByLabelText("Amount"), "10.00");
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(screen.getByText(/Conversion unavailable/i)).toBeInTheDocument();
    });

    // The form is still visible (not navigated away or reset), so the user
    // can retry without retyping. No expense list refetch is triggered.
    expect(screen.getByLabelText("Name")).toHaveValue("FX fail");
    expect(screen.getByRole("button", { name: "Log Expense" })).toBeEnabled();
  });
});
