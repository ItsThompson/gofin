import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";

import {
  findProRataPostCall,
  jsonResponse,
  mockFetch,
  mockTags,
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

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe("NewExpenseFeature - Pro-rata flow", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
    mockToastSuccess.mockReset();
    mockToastError.mockReset();
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

    await waitForFormBootstrap();

    await user.type(screen.getByLabelText("Name"), "Annual subscription");
    await user.type(screen.getByLabelText("Amount"), "120.00");

    const checkbox = screen.getByLabelText("Spread across months");
    await user.click(checkbox);

    // Leave months empty (don't type anything) to trigger validation
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(screen.getByText("Must be at least 2 months")).toBeInTheDocument();
    });
    expect(findProRataPostCall()).toBeUndefined();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("submits pro-rata expense and resets visible pro-rata state", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

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
      expect(mockToastSuccess).toHaveBeenCalledWith("Expense schedule saved");
    });
    expect(mockNavigate).not.toHaveBeenCalled();

    const proRataCall = findProRataPostCall();
    expect(proRataCall).toBeDefined();
    const body = JSON.parse(proRataCall![1].body);
    expect(body.totalAmount).toBe(12000);
    expect(body.months).toBe(3);
    expect(body.name).toBe("Annual subscription");
    expect(body.expenseType).toBe("savings");

    await waitFor(() => {
      expect(screen.getByLabelText("Spread across months")).not.toBeChecked();
      expect(screen.queryByLabelText("Number of months")).not.toBeInTheDocument();
      expect(screen.getByLabelText("Name")).toHaveValue("");
      expect(screen.getByLabelText("Amount")).toHaveValue(null);
      expect(screen.getByLabelText("essentials")).toBeChecked();
    });
  });

  it("uses the submitted request kind for in-flight pro-rata success toast", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    let resolvePost!: (response: unknown) => void;
    const pendingPost = new Promise((resolve) => {
      resolvePost = resolve;
    });

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/finance/prorata") && init?.method === "POST") {
        return pendingPost;
      }

      if (url.includes("/api/finance/tags")) {
        return jsonResponse({ tags: mockTags });
      }

      if (url.includes("/api/expenses/suggestions")) {
        return jsonResponse({ data: [], total: 0, page: 1, pageSize: 50, hasMore: false });
      }

      return jsonResponse({ message: "Unhandled request" }, 404);
    });

    await user.type(screen.getByLabelText("Name"), "Annual subscription");
    await user.type(screen.getByLabelText("Amount"), "120.00");
    await user.click(screen.getByLabelText("Spread across months"));
    await user.type(screen.getByLabelText("Number of months"), "3");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(findProRataPostCall()).toBeDefined();
    });
    const proRataCall = findProRataPostCall();
    const body = JSON.parse(proRataCall![1].body);
    expect(body).toMatchObject({
      name: "Annual subscription",
      totalAmount: 12000,
      months: 3,
    });

    await user.click(screen.getByLabelText("Spread across months"));

    resolvePost({
      ok: true,
      status: 201,
      json: () => Promise.resolve({ schedule: { id: "prorata-1" } }),
    });

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith("Expense schedule saved");
    });
    expect(mockToastSuccess).not.toHaveBeenCalledWith("Expense saved");
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("shows network error message and generic failure toast when network error occurs", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitForFormBootstrap();

    mockFetch.mockRejectedValueOnce(new TypeError("Network error"));

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.type(screen.getByLabelText("Amount"), "5.00");

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => {
      expect(
        screen.getByText("Connection lost. Check your internet and try again."),
      ).toBeInTheDocument();
    });
    expect(mockToastError).toHaveBeenCalledWith("Failed to save expense");
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
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
