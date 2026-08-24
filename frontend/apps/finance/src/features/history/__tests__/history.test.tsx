import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { HistoryFeature } from "../index";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockPeriods = [
  {
    id: "p3",
    userId: "user-1",
    year: 2026,
    month: 3,
    budgetAmount: 300000,
    reportingCurrency: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-03-01T00:00:00Z",
    updatedAt: "2026-03-01T00:00:00Z",
  },
  {
    id: "p2",
    userId: "user-1",
    year: 2026,
    month: 2,
    budgetAmount: 250000,
    reportingCurrency: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-02-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z",
  },
];

function mockPeriodsResponse() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ periods: mockPeriods }),
  });
}

function mockSummaryResponse(totalSpent: number) {
  return {
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        summary: {
          periodId: "p",
          year: 2026,
          month: 3,
          totalBudget: 300000,
          totalSpent,
          remaining: 300000 - totalSpent,
          daysInPeriod: 31,
          daysElapsed: 31,
          dailySpendRate: 0,
          budgetPace: 0,
          isOnTrack: true,
          essentials: { allocated: 150000, spent: 0, remaining: 150000, percentUsed: 0 },
          desires: { allocated: 90000, spent: 0, remaining: 90000, percentUsed: 0 },
          savings: { allocated: 60000, spent: 0, remaining: 60000, percentUsed: 0 },
        },
      }),
  };
}

function renderHistory() {
  return render(
    <MemoryRouter>
      <HistoryFeature />
    </MemoryRouter>,
  );
}

describe("HistoryFeature", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("renders loading state initially", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => {}));
    renderHistory();
    expect(screen.getByText("Loading history...")).toBeInTheDocument();
  });

  it("renders period list with spent and surplus data", async () => {
    mockPeriodsResponse();
    // Summary for each period
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(200000)); // March: $2000 spent
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000)); // Feb: $2800 spent

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("Budget History")).toBeInTheDocument();
    });

    // Both months should appear
    expect(screen.getByText("March 2026")).toBeInTheDocument();
    expect(screen.getByText("February 2026")).toBeInTheDocument();

    // March: spent $2000, budget $3000, surplus $1000
    expect(screen.getByText("Spent: $2,000.00")).toBeInTheDocument();
    expect(screen.getByText("Surplus: $1,000.00")).toBeInTheDocument();

    // Feb: spent $2800, budget $2500, deficit $300
    expect(screen.getByText("Spent: $2,800.00")).toBeInTheDocument();
    expect(screen.getByText("Deficit: $300.00")).toBeInTheDocument();
  });

  it("shows empty state when no periods exist", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: [] }),
    });

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("No budget periods yet.")).toBeInTheDocument();
    });
  });

  it("navigates to read-only dashboard when a period is clicked", async () => {
    mockPeriodsResponse();
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(200000));
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000));

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    // Click March 2026 period
    await userEvent.click(screen.getByTestId("period-row-2026-3"));

    // Should show read-only dashboard with Back button
    // The dashboard fetches: summary, by-tag, cumulative, expenses, comparison
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(200000)); // summary
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ tagSpending: [] }),
    }); // by-tag
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ points: [] }),
    }); // cumulative
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
    }); // expenses
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ code: "NOT_FOUND", message: "n/a" }),
    }); // comparison

    await waitFor(() => {
      expect(screen.getByText("Back to History")).toBeInTheDocument();
    });

    // Month name should appear in the header
    expect(screen.getByText("March 2026")).toBeInTheDocument();

    // Budget Settings button should NOT be visible (read-only mode)
    expect(screen.queryByLabelText("Budget Settings")).not.toBeInTheDocument();
  });

  it("renders the failed period as unavailable while surviving periods keep their values", async () => {
    mockPeriodsResponse();
    mockFetch.mockRejectedValueOnce(new Error("Network error")); // March
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000)); // Feb

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    // March failed: no fabricated figures anywhere in the list.
    expect(screen.getByText("Spent: unavailable")).toBeInTheDocument();
    expect(screen.getByText("Could not load this month")).toBeInTheDocument();
    expect(screen.queryByText("Spent: $0.00")).not.toBeInTheDocument();
    expect(screen.queryByText("Surplus: $3,000.00")).not.toBeInTheDocument();

    // February survived and still shows its real values.
    expect(screen.getByText("Spent: $2,800.00")).toBeInTheDocument();
    expect(screen.getByText("Deficit: $300.00")).toBeInTheDocument();
  });

  it("renders every row as unavailable when all summaries fail", async () => {
    mockPeriodsResponse();
    mockFetch.mockRejectedValue(new Error("Network error"));

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("Budget History")).toBeInTheDocument();
    });

    expect(screen.getByText("March 2026")).toBeInTheDocument();
    expect(screen.getByText("February 2026")).toBeInTheDocument();
    expect(screen.getAllByText("Spent: unavailable")).toHaveLength(2);
    expect(screen.queryByText(/^Spent: \$/)).not.toBeInTheDocument();
  });

  it("renders a genuine zero spend as a loaded value, not as unavailable", async () => {
    mockPeriodsResponse();
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(0)); // March: nothing spent
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000)); // Feb

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    expect(screen.getByText("Spent: $0.00")).toBeInTheDocument();
    expect(screen.getByText("Surplus: $3,000.00")).toBeInTheDocument();
    expect(screen.queryByText("Spent: unavailable")).not.toBeInTheDocument();
  });

  it("shows empty state when periods fetch fails (toast handles error)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () =>
        Promise.resolve({
          code: "INTERNAL_SERVER_ERROR",
          message: "Server error",
        }),
    });

    renderHistory();

    // On error, the page renders with empty data
    await waitFor(() => {
      expect(screen.getByText("No budget periods yet.")).toBeInTheDocument();
    });
  });

  // --- Reporting currency and mixed-currency guards ---

  it("formats each row with the period reporting currency, not user.currency", async () => {
    const jpyPeriods = [
      {
        ...mockPeriods[0],
        id: "p3-jpy",
        budgetAmount: 30000,
        reportingCurrency: "JPY",
      },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: jpyPeriods }),
    });
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(20000));

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    // JPY has 0 minor unit digits, so no decimals.
    expect(screen.getByText("Spent: ¥20,000")).toBeInTheDocument();
    expect(screen.getByText(/Budget:.*¥30,000/)).toBeInTheDocument();
  });

  it("hides amount delta for adjacent rows with different reporting currencies", async () => {
    const mixedPeriods = [
      {
        ...mockPeriods[0],
        id: "p3-usd",
        reportingCurrency: "USD",
      },
      {
        ...mockPeriods[1],
        id: "p2-eur",
        reportingCurrency: "EUR",
      },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: mixedPeriods }),
    });
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(200000)); // March USD
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(180000)); // Feb EUR

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    // The delta should be labeled not comparable.
    expect(screen.getByText(/not comparable/i)).toBeInTheDocument();
  });

  it("shows amount delta for adjacent rows with the same reporting currency", async () => {
    mockPeriodsResponse();
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(200000)); // March
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000)); // Feb

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    // Delta = 200000 - 280000 = -80000 => -$800.00
    expect(screen.getByText(/Δ.*\$800.00 from last/i)).toBeInTheDocument();
  });
});
