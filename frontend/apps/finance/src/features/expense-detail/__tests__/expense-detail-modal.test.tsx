import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { ExpenseDetailModal } from "@/features/expense-detail";
import type { Expense, Tag } from "@gofin/core";
import type { ExpenseSuggestionsResponse } from "../../expense-autocomplete";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockTags: Tag[] = [
  {
    id: "tag-food",
    name: "Food",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "tag-transport",
    name: "Transport",
    isDefault: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

const activeExpense: Expense = {
  id: "exp-1",
  userId: "user-1",
  name: "Groceries",
  amount: 5000,
  transactionCurrency: "USD",
  expenseType: "essentials",
  tagId: "tag-food",
  expenseDate: "2026-05-02",
  periodYear: 2026,
  periodMonth: 5,
  status: "active",
  isProRata: false,
  createdAt: "2026-05-02T10:00:00Z",
};

const correctedExpense: Expense = {
  id: "exp-original",
  userId: "user-1",
  name: "Old Coffee",
  amount: 500,
  transactionCurrency: "USD",
  expenseType: "desires",
  tagId: "tag-food",
  expenseDate: "2026-05-01",
  periodYear: 2026,
  periodMonth: 5,
  status: "corrected",
  isProRata: false,
  createdAt: "2026-05-01T08:00:00Z",
};

const correctionExpense: Expense = {
  id: "exp-correction",
  userId: "user-1",
  name: "Updated Coffee",
  amount: 600,
  transactionCurrency: "USD",
  expenseType: "desires",
  tagId: "tag-food",
  expenseDate: "2026-05-01",
  periodYear: 2026,
  periodMonth: 5,
  status: "active",
  correctsId: "exp-original",
  isProRata: false,
  createdAt: "2026-05-01T09:00:00Z",
};

const correctionChain: Expense[] = [correctedExpense, correctionExpense];

function mockExpenseAndHistory(
  expense: Expense,
  history: Expense[] = [expense],
) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ expense }),
  });
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ entries: history }),
  });
}

function mockExpenseSuggestions(
  overrides: Partial<ExpenseSuggestionsResponse> = {},
) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        data: [
          {
            name: "Train Pass",
            amount: 1234,
            currency: "USD",
            expenseType: "desires",
            tagId: "tag-transport",
            frequency: 4,
            lastUsedAt: "2026-05-28T19:02:11Z",
            recencyBucket: "last_7_days",
            frecencyScore: 22,
          },
        ],
        total: 1,
        page: 1,
        pageSize: 50,
        hasMore: false,
        ...overrides,
      }),
  });
}

function mockCorrectionSuccess() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 201,
    json: () =>
      Promise.resolve({
        expense: {
          ...activeExpense,
          id: "exp-new-correction",
          name: "Corrected Groceries",
          amount: 6000,
          correctsId: "exp-1",
        },
      }),
  });
}

const defaultProps = {
  currency: "USD",
  tags: mockTags,
  currentYear: 2026,
  currentMonth: 5,
  onClose: vi.fn(),
  onCorrected: vi.fn(),
};

function renderModal(expenseId: string | null = "exp-1") {
  return render(
    <MemoryRouter>
      <ExpenseDetailModal expenseId={expenseId} {...defaultProps} />
    </MemoryRouter>,
  );
}

describe("ExpenseDetailModal", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    defaultProps.onClose.mockReset();
    defaultProps.onCorrected.mockReset();
  });

  describe("detail view", () => {
    it("renders expense details when loaded", async () => {
      mockExpenseAndHistory(activeExpense);
      renderModal();

      await waitFor(() => {
        expect(screen.getByText("Expense Detail")).toBeInTheDocument();
      });

      expect(screen.getByText("Groceries")).toBeInTheDocument();
      expect(screen.getByText("$50.00")).toBeInTheDocument();
      expect(screen.getByText("essentials")).toBeInTheDocument();
      expect(screen.getByText("Food")).toBeInTheDocument();
      expect(screen.getByText("2026-05-02")).toBeInTheDocument();
    });

    it("shows loading state initially", () => {
      mockExpenseAndHistory(activeExpense);
      renderModal();

      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });

    it("does not render when expenseId is null", () => {
      const { container } = renderModal(null);
      expect(container.querySelector("[role='dialog']")).toBeNull();
    });

    it("shows correct button for active expense in current period", async () => {
      mockExpenseAndHistory(activeExpense);
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });
    });

    it("does not show correct button for corrected expense", async () => {
      mockExpenseAndHistory(correctedExpense, correctionChain);
      renderModal("exp-original");

      await waitFor(() => {
        expect(screen.getByText("Expense Detail")).toBeInTheDocument();
      });

      expect(
        screen.queryByText("Correct This Expense"),
      ).not.toBeInTheDocument();
    });

    it("does not show correct button for expense from a past period", async () => {
      const pastExpense: Expense = {
        ...activeExpense,
        periodYear: 2026,
        periodMonth: 4, // Past period
      };
      mockExpenseAndHistory(pastExpense);
      renderModal();

      await waitFor(() => {
        expect(screen.getByText("Expense Detail")).toBeInTheDocument();
      });

      expect(
        screen.queryByText("Correct This Expense"),
      ).not.toBeInTheDocument();
    });
  });

  describe("correction notices", () => {
    it("shows 'was corrected' notice when expense has been corrected", async () => {
      mockExpenseAndHistory(correctedExpense, correctionChain);
      renderModal("exp-original");

      await waitFor(() => {
        expect(
          screen.getByText(/This expense was corrected/),
        ).toBeInTheDocument();
      });
    });

    it("shows 'corrects expense' notice when expense is a correction", async () => {
      mockExpenseAndHistory(correctionExpense, correctionChain);
      renderModal("exp-correction");

      await waitFor(() => {
        expect(
          screen.getByText(/This corrects expense/),
        ).toBeInTheDocument();
      });
    });
  });

  describe("correction history timeline", () => {
    it("shows correction history when corrections exist", async () => {
      mockExpenseAndHistory(correctedExpense, correctionChain);
      renderModal("exp-original");

      await waitFor(() => {
        expect(screen.getByText("Correction History")).toBeInTheDocument();
      });

      expect(screen.getByText("Original")).toBeInTheDocument();
      expect(screen.getByText("Correction 1")).toBeInTheDocument();
    });

    it("does not show correction history for standalone expense", async () => {
      mockExpenseAndHistory(activeExpense);
      renderModal();

      await waitFor(() => {
        expect(screen.getByText("Expense Detail")).toBeInTheDocument();
      });

      expect(
        screen.queryByText("Correction History"),
      ).not.toBeInTheDocument();
    });

    it("shows changes between entries in the timeline", async () => {
      mockExpenseAndHistory(correctedExpense, correctionChain);
      renderModal("exp-original");

      await waitFor(() => {
        expect(screen.getByText("Correction History")).toBeInTheDocument();
      });

      // The correction changed name and amount
      expect(
        screen.getByText(/Name: Old Coffee → Updated Coffee/),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Amount: \$5\.00 → \$6\.00/),
      ).toBeInTheDocument();
    });
  });

  describe("correction form", () => {
    it("opens correction form when clicking Correct button", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      expect(screen.getByText("Correct Expense")).toBeInTheDocument();
      expect(screen.getByText("Save Correction")).toBeInTheDocument();
    });

    it("pre-fills form with current expense values", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      const nameInput = screen.getByLabelText("Name") as HTMLInputElement;
      const amountInput = screen.getByLabelText("Amount") as HTMLInputElement;
      const dateInput = screen.getByLabelText("Date") as HTMLInputElement;

      expect(nameInput.value).toBe("Groceries");
      expect(amountInput.value).toBe("50.00");
      expect(dateInput.value).toBe("2026-05-02");
    });

    it("autofills correction fields from an explicit expense suggestion selection", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      const nameInput = screen.getByLabelText("Name") as HTMLInputElement;
      await user.clear(nameInput);
      await user.type(nameInput, "tra");

      await waitFor(() => {
        expect(screen.getByText("Train Pass")).toBeInTheDocument();
      });

      await user.click(screen.getByText("Train Pass"));

      expect(nameInput.value).toBe("Train Pass");
      expect((screen.getByLabelText("Amount") as HTMLInputElement).value).toBe(
        "12.34",
      );
      expect(screen.getByLabelText("desires")).toBeChecked();
      expect((screen.getByLabelText("Tag") as HTMLSelectElement).value).toBe(
        "tag-transport",
      );
      expect((screen.getByLabelText("Date") as HTMLInputElement).value).toBe(
        "2026-05-02",
      );
    });

    it("submits correction and calls onCorrected", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      // Modify the name
      const nameInput = screen.getByLabelText("Name") as HTMLInputElement;
      await user.clear(nameInput);
      await user.type(nameInput, "Corrected Groceries");

      // Mock the correction POST + subsequent re-fetch
      mockCorrectionSuccess();
      // After correction, the modal re-fetches expense + history
      mockExpenseAndHistory(activeExpense);

      await user.click(screen.getByText("Save Correction"));

      await waitFor(() => {
        expect(defaultProps.onCorrected).toHaveBeenCalledTimes(1);
      });
    });

    it("shows cancel button that returns to detail view", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));
      expect(screen.getByText("Correct Expense")).toBeInTheDocument();

      await user.click(screen.getByText("Cancel"));
      expect(screen.getByText("Expense Detail")).toBeInTheDocument();
    });

    it("validates required fields", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      // Clear the name field
      const nameInput = screen.getByLabelText("Name") as HTMLInputElement;
      await user.clear(nameInput);

      await user.click(screen.getByText("Save Correction"));

      expect(screen.getByText("Name is required")).toBeInTheDocument();
    });

    it("shows ALREADY_CORRECTED error message", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      // Mock a 409 response
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: () =>
          Promise.resolve({
            code: "ALREADY_CORRECTED",
            message: "this expense has already been corrected",
          }),
      });

      await user.click(screen.getByText("Save Correction"));

      await waitFor(() => {
        expect(
          screen.getByText(/already been corrected/),
        ).toBeInTheDocument();
      });
    });

    it("shows PERIOD_LOCKED error message", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(
          screen.getByText("Correct This Expense"),
        ).toBeInTheDocument();
      });

      mockExpenseSuggestions();

      await user.click(screen.getByText("Correct This Expense"));

      // Mock a 403 response
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        json: () =>
          Promise.resolve({
            code: "PERIOD_LOCKED",
            message: "cannot correct expenses from a past period",
          }),
      });

      await user.click(screen.getByText("Save Correction"));

      await waitFor(() => {
        expect(
          screen.getByText(/cannot correct expenses from a past period/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("close behavior", () => {
    it("calls onClose when close button is clicked", async () => {
      mockExpenseAndHistory(activeExpense);
      const user = userEvent.setup();
      renderModal();

      await waitFor(() => {
        expect(screen.getByText("Expense Detail")).toBeInTheDocument();
      });

      await user.click(screen.getByLabelText("Close"));
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });
  });
});
