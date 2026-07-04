import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";

import type { ExpenseSuggestionsResponse } from "../../expense-autocomplete";
import {
  jsonResponse,
  mockFetch,
  mockSuggestions,
  mockTags,
} from "../__mocks__";
import {
  getSubmittedExpenseRequest,
  renderNewExpense as renderNewExpenseFeature,
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

function renderNewExpense(
  suggestions: ExpenseSuggestionsResponse = mockSuggestions,
  tags = mockTags,
) {
  return renderNewExpenseFeature({ suggestions, tags });
}

function renderNewExpenseWithFetchHandler(
  handler: (url: string, init?: RequestInit) => Promise<unknown>,
) {
  return renderNewExpenseFeature({ fetchHandler: handler });
}

describe("NewExpenseFeature autocomplete integration", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockNavigate.mockReset();
    mockToastSuccess.mockReset();
    mockToastError.mockReset();
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

  it("keeps the suggestion list hidden and disables browser autocomplete before typing", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    const nameInput = screen.getByLabelText("Name");
    await user.click(nameInput);

    expect(nameInput).toHaveAttribute("autocomplete", "off");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.queryByText("No matching expenses")).not.toBeInTheDocument();
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

  it("places load more after visible suggestions when the latest page has more results", async () => {
    const user = userEvent.setup();
    renderNewExpense({ ...mockSuggestions, hasMore: true });

    await user.type(screen.getByLabelText("Name"), "Coffee");

    const listbox = await screen.findByRole("listbox");
    const options = within(listbox).getAllByRole("option");
    expect(options.map((option) => option.textContent)).toEqual([
      "Coffee ShopFrecency score: 42",
      "Coffee BeansFrecency score: 31",
      "Load more suggestions",
    ]);
  });

  it("shows no-match state followed by load more when more pages are available", async () => {
    const user = userEvent.setup();
    renderNewExpense({ ...mockSuggestions, hasMore: true });

    await user.type(screen.getByLabelText("Name"), "zzzz");

    expect(await screen.findByText("No matching expenses")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Load more suggestions" })).toBeInTheDocument();
  });

  it("hides load more when the latest page has no more results", async () => {
    const user = userEvent.setup();
    renderNewExpense({ ...mockSuggestions, hasMore: false });

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await screen.findByText("Coffee Shop");

    expect(screen.queryByRole("option", { name: "Load more suggestions" })).not.toBeInTheDocument();
  });

  it("loads the next page by pointer and refreshes fuzzy matches without dropping existing suggestions", async () => {
    const user = userEvent.setup();
    const secondPageSuggestion = {
      ...mockSuggestions.data[0],
      name: "Custom Coffee Roaster",
      amount: 2200,
      frecencyScore: 18,
    };

    renderNewExpenseWithFetchHandler((url: string) => {
      if (url.includes("/api/finance/tags")) {
        return jsonResponse({ tags: mockTags });
      }

      if (url.includes("/api/expenses/suggestions?page=1")) {
        return jsonResponse({ ...mockSuggestions, hasMore: true });
      }

      if (url.includes("/api/expenses/suggestions?page=2")) {
        return jsonResponse({
          data: [secondPageSuggestion],
          total: 3,
          page: 2,
          pageSize: 50,
          hasMore: false,
        });
      }

      return jsonResponse({ message: "Unhandled request" }, 404);
    });

    await user.type(screen.getByLabelText("Name"), "Custom");
    expect(await screen.findByText("No matching expenses")).toBeInTheDocument();

    await user.click(screen.getByRole("option", { name: "Load more suggestions" }));

    expect(await screen.findByText("Custom Coffee Roaster")).toBeInTheDocument();
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/expenses/suggestions?page=2&pageSize=50",
      expect.objectContaining({ credentials: "include" }),
    );

    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Coffee");

    expect(await screen.findByText("Coffee Shop")).toBeInTheDocument();
    expect(screen.getByText("Custom Coffee Roaster")).toBeInTheDocument();
  });

  it("loads the next page by keyboard without selecting a suggestion", async () => {
    const user = userEvent.setup();
    const secondPageSuggestion = {
      ...mockSuggestions.data[0],
      name: "Coffee Cart",
      amount: 900,
      frecencyScore: 12,
    };

    renderNewExpenseWithFetchHandler((url: string) => {
      if (url.includes("/api/finance/tags")) {
        return jsonResponse({ tags: mockTags });
      }

      if (url.includes("/api/expenses/suggestions?page=1")) {
        return jsonResponse({ ...mockSuggestions, data: [], hasMore: true });
      }

      if (url.includes("/api/expenses/suggestions?page=2")) {
        return jsonResponse({ data: [secondPageSuggestion], total: 1, page: 2, pageSize: 50, hasMore: false });
      }

      return jsonResponse({ message: "Unhandled request" }, 404);
    });

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));
    await user.type(screen.getByLabelText("Amount"), "7.77");
    await user.type(screen.getByLabelText("Name"), "Coffee");
    await screen.findByRole("option", { name: "Load more suggestions" });

    await user.keyboard("{ArrowDown}{Enter}");

    expect(await screen.findByText("Coffee Cart")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Coffee");
    expect(screen.getByLabelText("Amount")).toHaveValue(7.77);
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

    await waitFor(() => expect(mockToastSuccess).toHaveBeenCalledWith("Expense saved"));
    expect(mockNavigate).not.toHaveBeenCalled();

    expect(getSubmittedExpenseRequest().name).toBe("Custom Coffee");
  });

  it("keeps the expense form usable when the initial suggestions request fails", async () => {
    const user = userEvent.setup();

    renderNewExpenseWithFetchHandler((url: string, init?: RequestInit) => {
      if (url.includes("/api/finance/tags")) {
        return jsonResponse({ tags: mockTags });
      }

      if (url.includes("/api/expenses/suggestions")) {
        return jsonResponse({ code: "internal_server_error", message: "failed" }, 500);
      }

      if (url.includes("/api/expenses") && init?.method === "POST") {
        return jsonResponse({ expense: { id: "exp-1", name: "Manual Expense" } }, 201);
      }

      return jsonResponse({ message: "Unhandled request" }, 404);
    });

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));

    await user.type(screen.getByLabelText("Name"), "Manual Expense");
    await user.type(screen.getByLabelText("Amount"), "8.25");
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => expect(mockToastSuccess).toHaveBeenCalledWith("Expense saved"));
    expect(mockNavigate).not.toHaveBeenCalled();
    expect(getSubmittedExpenseRequest().name).toBe("Manual Expense");
  });

  it("keeps loaded suggestions selectable when a later load-more request fails", async () => {
    const user = userEvent.setup();

    renderNewExpenseWithFetchHandler((url: string) => {
      if (url.includes("/api/finance/tags")) {
        return jsonResponse({ tags: mockTags });
      }

      if (url.includes("/api/expenses/suggestions?page=1")) {
        return jsonResponse({ ...mockSuggestions, hasMore: true });
      }

      if (url.includes("/api/expenses/suggestions?page=2")) {
        return jsonResponse({ code: "internal_server_error", message: "failed" }, 500);
      }

      return jsonResponse({ message: "Unhandled request" }, 404);
    });

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));
    await user.type(screen.getByLabelText("Name"), "Coffee");
    await screen.findByText("Coffee Shop");

    await user.click(screen.getByRole("option", { name: "Load more suggestions" }));
    await user.click(await screen.findByText("Coffee Shop"));

    expect(screen.getByLabelText("Name")).toHaveValue("Coffee Shop");
    expect(screen.getByLabelText("Amount")).toHaveValue(4.5);
    expect(screen.getByLabelText("desires")).toBeChecked();
  });

  it("autofills name, amount, type, and existing tag when clicking a suggestion", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));
    const dateBeforeSelection = (screen.getByLabelText("Date") as HTMLInputElement).value;

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.click(await screen.findByText("Coffee Shop"));

    expect(screen.getByLabelText("Name")).toHaveValue("Coffee Shop");
    expect(screen.getByLabelText("Amount")).toHaveValue(4.5);
    expect(screen.getByLabelText("desires")).toBeChecked();
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-food");
    expect(screen.getByLabelText("Date")).toHaveValue(dateBeforeSelection);
    expect(screen.getByLabelText("Spread across months")).not.toBeChecked();
    expect(screen.queryByLabelText("Number of months")).not.toBeInTheDocument();
  });

  it("applies the same autofill behavior with ArrowDown and Enter", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await screen.findByText("Coffee Shop");
    await user.keyboard("{ArrowDown}{Enter}");

    expect(screen.getByLabelText("Name")).toHaveValue("Coffee Shop");
    expect(screen.getByLabelText("Amount")).toHaveValue(4.5);
    expect(screen.getByLabelText("desires")).toBeChecked();
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-food");
  });

  it("does not autofill on highlight movement, plain Enter, or blur", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));
    await user.type(screen.getByLabelText("Amount"), "9.99");
    await user.click(screen.getByLabelText("essentials"));
    await user.selectOptions(screen.getByLabelText("Tag"), "tag-bills");

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await screen.findByText("Coffee Shop");
    await user.keyboard("{ArrowDown}");

    expect(screen.getByLabelText("Amount")).toHaveValue(9.99);
    expect(screen.getByLabelText("essentials")).toBeChecked();
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/expenses") && init?.method === "POST") {
        return new Promise(() => {});
      }

      if (url.includes("/api/finance/tags")) {
        return jsonResponse({ tags: mockTags });
      }

      if (url.includes("/api/expenses/suggestions")) {
        return jsonResponse(mockSuggestions);
      }

      return jsonResponse({ message: "Unhandled request" }, 404);
    });

    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Plain Coffee{Enter}");
    await user.tab();

    expect(screen.getByLabelText("Name")).toHaveValue("Plain Coffee");
    expect(screen.getByLabelText("Amount")).toHaveValue(9.99);
    expect(screen.getByLabelText("essentials")).toBeChecked();
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");
  });

  it("leaves the current tag unchanged when the selected suggestion tag is stale", async () => {
    const user = userEvent.setup();
    renderNewExpense(mockSuggestions, [mockTags[0]]);

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.click(await screen.findByText("Coffee Shop"));

    expect(screen.getByLabelText("Name")).toHaveValue("Coffee Shop");
    expect(screen.getByLabelText("Amount")).toHaveValue(4.5);
    expect(screen.getByLabelText("desires")).toBeChecked();
    expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills");
  });

  it("submits visible edited values after autofill", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await waitFor(() => expect(screen.getByLabelText("Tag")).toHaveValue("tag-bills"));

    await user.type(screen.getByLabelText("Name"), "Coffee");
    await user.click(await screen.findByText("Coffee Shop"));
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Edited Coffee");
    await user.clear(screen.getByLabelText("Amount"));
    await user.type(screen.getByLabelText("Amount"), "6.25");
    await user.click(screen.getByLabelText("essentials"));
    await user.selectOptions(screen.getByLabelText("Tag"), "tag-bills");
    await user.clear(screen.getByLabelText("Date"));
    await user.type(screen.getByLabelText("Date"), "2026-05-02");
    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    await waitFor(() => expect(mockToastSuccess).toHaveBeenCalledWith("Expense saved"));
    expect(mockNavigate).not.toHaveBeenCalled();

    expect(getSubmittedExpenseRequest()).toMatchObject({
      name: "Edited Coffee",
      amount: 625,
      expenseType: "essentials",
      tagId: "tag-bills",
      expenseDate: "2026-05-02",
    });
  });

  it("keeps existing required-name validation for typed input", async () => {
    const user = userEvent.setup();
    renderNewExpense();

    await user.click(screen.getByRole("button", { name: "Log Expense" }));

    expect(screen.getByText("Name is required")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockToastError).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
