import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { DashboardPage } from "@/pages/DashboardPage";
import type { User } from "@gofin/types";

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

const mockPeriod = {
  id: "period-abc",
  userId: "user-1",
  year: 2026,
  month: 5,
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
};

const mockDefaults = {
  userId: "user-1",
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function mockPeriodFound() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ period: mockPeriod }),
  });
}

function mockPeriodNotFound() {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 404,
    json: () =>
      Promise.resolve({
        code: "PERIOD_NOT_FOUND",
        message: "No budget period found for 2026-05",
      }),
  });
}

function mockDefaultsFound() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ defaults: mockDefaults }),
  });
}

function mockDefaultsNotFound() {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 404,
    json: () =>
      Promise.resolve({
        code: "NOT_FOUND",
        message: "Default settings not found",
      }),
  });
}

function mockServerError(message: string) {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 500,
    json: () =>
      Promise.resolve({
        code: "INTERNAL_SERVER_ERROR",
        message,
      }),
  });
}

function mockCreatePeriodSuccess() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 201,
    json: () => Promise.resolve({ period: mockPeriod }),
  });
}

function renderDashboard(user: User = mockUser) {
  return render(
    <MemoryRouter>
      <DashboardPage user={user} />
    </MemoryRouter>,
  );
}

describe("DashboardPage", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("renders loading state initially", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => {}));
    renderDashboard();
    expect(screen.getByText("Loading dashboard...")).toBeInTheDocument();
  });

  describe("active period exists", () => {
    it("renders summary bar with budget values", async () => {
      mockPeriodFound();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      expect(screen.getByText("Total Budget")).toBeInTheDocument();
      // $3,000.00 appears in both Total Budget and Remaining cards
      expect(screen.getAllByText("$3,000.00")).toHaveLength(2);
      expect(screen.getByText("Total Spent")).toBeInTheDocument();
      expect(screen.getByText("$0.00")).toBeInTheDocument();
      expect(screen.getByText("Remaining")).toBeInTheDocument();
      expect(screen.getByText("Days Left")).toBeInTheDocument();
    });

    it("renders empty state with CTA to log expense", async () => {
      mockPeriodFound();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("No expenses yet")).toBeInTheDocument();
      });

      const ctaLink = screen.getByRole("link", {
        name: /log your first expense/i,
      });
      expect(ctaLink).toBeInTheDocument();
      expect(ctaLink).toHaveAttribute("href", "/expenses/new");
    });

    it("displays currency symbol from user profile", async () => {
      mockPeriodFound();
      renderDashboard({ ...mockUser, currency: "EUR" });

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      // €3,000.00 appears in Total Budget and Remaining
      expect(screen.getAllByText("€3,000.00")).toHaveLength(2);
      expect(screen.getByText("€0.00")).toBeInTheDocument();
    });

    it("color-codes remaining balance green when > 30%", async () => {
      mockPeriodFound();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      // Remaining is $3,000 out of $3,000 = 100%, should be green
      // The two $3,000.00 elements: one without color (Total Budget), one with green (Remaining)
      const amounts = screen.getAllByText("$3,000.00");
      const greenAmount = amounts.find((element) =>
        element.className.includes("text-green"),
      );
      expect(greenAmount).toBeDefined();
    });
  });

  describe("no period exists (PERIOD_NOT_FOUND)", () => {
    it("shows creation prompt with default values", async () => {
      mockPeriodNotFound();
      mockDefaultsFound();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      // Fields should be pre-filled with defaults
      const budgetInput = screen.getByLabelText(
        "Monthly Budget",
      ) as HTMLInputElement;
      expect(budgetInput.value).toBe("3000");

      const essentialsInput = screen.getByLabelText(
        "Essentials %",
      ) as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");

      const desiresInput = screen.getByLabelText(
        "Desires %",
      ) as HTMLInputElement;
      expect(desiresInput.value).toBe("30");

      const savingsInput = screen.getByLabelText(
        "Savings %",
      ) as HTMLInputElement;
      expect(savingsInput.value).toBe("20");
    });

    it("shows zero-budget warning when default budget is $0", async () => {
      mockPeriodNotFound();
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            defaults: { ...mockDefaults, budgetAmount: 0 },
          }),
      });
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/no budget configured/i)).toBeInTheDocument();
      });
    });

    it("uses fallback defaults when defaults endpoint fails", async () => {
      mockPeriodNotFound();
      mockDefaultsNotFound();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      // Fallback: 50/30/20 split, empty budget
      const essentialsInput = screen.getByLabelText(
        "Essentials %",
      ) as HTMLInputElement;
      expect(essentialsInput.value).toBe("50");
    });

    it("validates E/D/S split sums to 100%", async () => {
      mockPeriodNotFound();
      mockDefaultsFound();

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      // Change savings to 19 (total = 99%)
      const savingsInput = screen.getByLabelText("Savings %");
      await user.clear(savingsInput);
      await user.type(savingsInput, "19");

      // Submit
      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      expect(
        screen.getByText(/percentages must sum to 100%/i),
      ).toBeInTheDocument();
    });

    it("creates period on valid submission and shows dashboard", async () => {
      mockPeriodNotFound();
      mockDefaultsFound();
      mockCreatePeriodSuccess();

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      // After creation, dashboard should render with summary bar
      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
        expect(screen.getAllByText("$3,000.00")).toHaveLength(2);
      });
    });

    it("does NOT change stored defaults when user overrides values", async () => {
      mockPeriodNotFound();
      mockDefaultsFound();
      mockCreatePeriodSuccess();

      const user = userEvent.setup();
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText(/set up/i)).toBeInTheDocument();
      });

      // Override budget to $5,000
      const budgetInput = screen.getByLabelText("Monthly Budget");
      await user.clear(budgetInput);
      await user.type(budgetInput, "5000");

      const submitButton = screen.getByRole("button", { name: /create/i });
      await user.click(submitButton);

      // Verify the POST was to /api/finance/periods (not /api/finance/defaults)
      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });

      const createCall = mockFetch.mock.calls.find(
        (call) =>
          typeof call[0] === "string" &&
          call[0].includes("/api/finance/periods") &&
          call[1]?.method === "POST",
      );
      expect(createCall).toBeDefined();
      const body = JSON.parse(createCall![1].body);
      expect(body.budgetAmount).toBe(500000); // $5,000 in cents

      // No call to PUT /api/finance/defaults should have been made
      const defaultsUpdateCall = mockFetch.mock.calls.find(
        (call) =>
          typeof call[0] === "string" &&
          call[0].includes("/api/finance/defaults") &&
          call[1]?.method === "PUT",
      );
      expect(defaultsUpdateCall).toBeUndefined();
    });
  });

  describe("error state", () => {
    it("renders error state on server error", async () => {
      mockServerError("Database connection failed");
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Error")).toBeInTheDocument();
      });

      expect(
        screen.getByText("Database connection failed"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /retry/i }),
      ).toBeInTheDocument();
    });

    it("retries fetch on retry button click", async () => {
      mockServerError("Temporary failure");
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Error")).toBeInTheDocument();
      });

      // Retry succeeds
      mockPeriodFound();
      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: /retry/i }));

      await waitFor(() => {
        expect(screen.getByText("Dashboard")).toBeInTheDocument();
      });
    });
  });
});
