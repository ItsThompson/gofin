import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { ExpenseDetailModal } from "@/features/expense-detail";
import type { Expense, Tag } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockTags: Tag[] = [
  { id: "tag-food", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-transport", name: "Transport", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
];

const activeExpense: Expense = {
  id: "exp-1",
  userId: "user-1",
  name: "Groceries",
  transactionCurrencyCode: "USD",
  originalTransactionAmountInMinorUnits: 5000,
  reportingAmountInMinorUnits: 5000,
  reportingCurrencyCode: "USD",
  expenseType: "essentials",
  tagId: "tag-food",
  expenseDateIso: "2026-05-02",
  periodYear: 2026,
  periodMonth: 5,
  status: "active",
  isProRata: false,
  createdAt: "2026-05-02T10:00:00Z",
};

function mockExpenseAndHistory(expense: Expense) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ expense }),
  });
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ entries: [expense] }),
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

function renderModal(expenseId: string = "exp-1") {
  return render(
    <MemoryRouter>
      <ExpenseDetailModal expenseId={expenseId} {...defaultProps} />
    </MemoryRouter>,
  );
}

describe("ExpenseDetailModal - Correction form field changes", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    defaultProps.onClose.mockReset();
    defaultProps.onCorrected.mockReset();
  });

  it("allows changing expense type in correction form", async () => {
    mockExpenseAndHistory(activeExpense);
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Correct This Expense")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Correct This Expense"));

    // Default is "essentials", change to "desires"
    const desiresRadio = screen.getByLabelText("desires") as HTMLInputElement;
    await user.click(desiresRadio);
    expect(desiresRadio.checked).toBe(true);

    const essentialsRadio = screen.getByLabelText("essentials") as HTMLInputElement;
    expect(essentialsRadio.checked).toBe(false);
  });

  it("allows changing tag in correction form", async () => {
    mockExpenseAndHistory(activeExpense);
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Correct This Expense")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Correct This Expense"));

    const tagSelect = screen.getByLabelText("Tag") as HTMLSelectElement;
    expect(tagSelect.value).toBe("tag-food");

    await user.selectOptions(tagSelect, "tag-transport");
    expect(tagSelect.value).toBe("tag-transport");
  });

  it("allows changing date in correction form", async () => {
    mockExpenseAndHistory(activeExpense);
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Correct This Expense")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Correct This Expense"));

    const dateInput = screen.getByLabelText("Date") as HTMLInputElement;
    expect(dateInput.value).toBe("2026-05-02");

    await user.clear(dateInput);
    await user.type(dateInput, "2026-05-05");
    expect(dateInput.value).toBe("2026-05-05");
  });

  it("allows changing amount in correction form", async () => {
    mockExpenseAndHistory(activeExpense);
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Correct This Expense")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Correct This Expense"));

    const amountInput = screen.getByLabelText("Amount") as HTMLInputElement;
    expect(amountInput.value).toBe("50.00");

    await user.clear(amountInput);
    await user.type(amountInput, "75.50");
    // Number inputs drop trailing zero
    expect(amountInput.value).toBe("75.5");
  });

  it("validates amount must be positive", async () => {
    mockExpenseAndHistory(activeExpense);
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Correct This Expense")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Correct This Expense"));

    const amountInput = screen.getByLabelText("Amount");
    await user.clear(amountInput);

    await user.click(screen.getByText("Save Correction"));

    expect(screen.getByText("Amount must be greater than 0")).toBeInTheDocument();
  });

  it("shows generic error when correction API throws non-API error", async () => {
    mockExpenseAndHistory(activeExpense);
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Correct This Expense")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Correct This Expense"));

    // Mock a network error (non-ApiRequestError)
    mockFetch.mockRejectedValueOnce(new TypeError("Network error"));

    await user.click(screen.getByText("Save Correction"));

    await waitFor(() => {
      expect(
        screen.getByText("Connection lost. Check your internet and try again."),
      ).toBeInTheDocument();
    });
  });

  it("shows error when expense detail fetch fails", async () => {
    // Mock the initial expense fetch to fail
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message: "Server error" }),
    });

    renderModal();

    await waitFor(() => {
      expect(screen.getByText("Failed to load expense details.")).toBeInTheDocument();
    });
  });
});
