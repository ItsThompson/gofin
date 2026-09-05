import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { HistoryFeature } from "../index";
import type { BudgetPeriod } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockPeriods: BudgetPeriod[] = [
  {
    id: "period-march",
    userId: "user-1",
    year: 2026,
    month: 3,
    budgetAmount: 300000,
    reportingCurrencyCode: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-03-01T00:00:00Z",
    updatedAt: "2026-03-01T00:00:00Z",
  },
];

function mockPeriodsAndSummary() {
  // Periods list
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ periods: mockPeriods }),
  });
  // Summary for the period
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        summary: {
          periodId: "period-march",
          year: 2026,
          month: 3,
          totalBudget: 300000,
          totalSpent: 200000,
          remaining: 100000,
          daysInPeriod: 31,
          daysElapsed: 31,
          dailySpendRate: 6451,
          budgetPace: 9677,
          isOnTrack: true,
          essentials: { allocated: 150000, spent: 100000, remaining: 50000, percentUsed: 66.67 },
          desires: { allocated: 90000, spent: 60000, remaining: 30000, percentUsed: 66.67 },
          savings: { allocated: 60000, spent: 40000, remaining: 20000, percentUsed: 66.67 },
        },
      }),
  });
}

function renderHistory() {
  return render(
    <MemoryRouter>
      <HistoryFeature />
    </MemoryRouter>,
  );
}

describe("HistoryFeature - back navigation", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("returns to period list when Back to History is clicked", async () => {
    mockPeriodsAndSummary();
    renderHistory();

    await waitFor(() => {
      expect(screen.getByText("March 2026")).toBeInTheDocument();
    });

    // Click to view the period detail
    await userEvent.click(screen.getByTestId("period-row-2026-3"));

    // Mock the dashboard data calls for the read-only view
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          summary: {
            periodId: "period-march",
            totalBudget: 300000,
            totalSpent: 200000,
            remaining: 100000,
            daysInPeriod: 31,
            daysElapsed: 31,
            isOnTrack: true,
            essentials: { allocated: 150000, spent: 100000, remaining: 50000, percentUsed: 66.67 },
            desires: { allocated: 90000, spent: 60000, remaining: 30000, percentUsed: 66.67 },
            savings: { allocated: 60000, spent: 40000, remaining: 20000, percentUsed: 66.67 },
          },
        }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ tagSpending: [] }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ points: [] }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ code: "NOT_FOUND", message: "n/a" }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ schedules: [] }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ trends: [] }),
    });

    await waitFor(() => {
      expect(screen.getByText("Back to History")).toBeInTheDocument();
    });

    // Click Back to History
    await userEvent.click(screen.getByText("Back to History"));

    // Should return to the period list
    await waitFor(() => {
      expect(screen.getByText("Budget History")).toBeInTheDocument();
    });
  });
});
