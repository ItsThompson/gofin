import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { HistoryFeature } from "../index";
import type { User } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockUser: User = {
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockPeriods = [
  {
    id: "p3",
    userId: "user-1",
    year: 2026,
    month: 3,
    budgetAmount: 300000,
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
      <HistoryFeature user={mockUser} />
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
});
