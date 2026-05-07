import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { useAuthStore } from "@/stores/auth-store";

const mockFetch = vi.fn();
global.fetch = mockFetch;

function resetStore(overrides: Record<string, unknown> = {}) {
  useAuthStore.setState({
    user: null,
    isAuthenticated: false,
    isAdmin: false,
    isAssuming: false,
    originalAdminUser: null,
    isLoading: false,
    ...overrides,
  });
}

const newUser = {
  id: "2",
  username: "newuser",
  email: "new@test.com",
  role: "user" as const,
  currency: "USD",
  hasCompletedOnboarding: false,
  createdAt: "2026-01-01",
};

async function importOnboardingPage() {
  const mod = await import("@/routes/onboarding");
  return mod.default;
}

function renderOnboarding(OnboardingPage: React.ComponentType) {
  const router = createMemoryRouter(
    [
      { path: "/onboarding", element: <OnboardingPage /> },
      { path: "/dashboard", element: <div>Dashboard page</div> },
    ],
    { initialEntries: ["/onboarding"] },
  );
  return render(<RouterProvider router={router} />);
}

describe("OnboardingPage - skip and navigation flows", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "No session" }),
    });
  });

  it("skips currency step and advances to budget step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Welcome → Currency
    fireEvent.click(screen.getByText("Get started"));
    expect(screen.getByText("Default Currency")).toBeInTheDocument();

    // Skip currency step
    fireEvent.click(screen.getByText("Skip (use USD)"));
    expect(screen.getByText("Monthly Budget")).toBeInTheDocument();
  });

  it("skips budget step and advances to split step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Welcome → Currency → Budget
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    expect(screen.getByText("Monthly Budget")).toBeInTheDocument();

    // Skip budget step
    fireEvent.click(screen.getByText("Skip ($0 budget)"));
    expect(screen.getByText("E/D/S Split")).toBeInTheDocument();
  });

  it("skips split step and submits with defaults", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    // Mock both API calls as successful
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ defaults: { userId: "2" } }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: { ...newUser, hasCompletedOnboarding: true } }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: { ...newUser, hasCompletedOnboarding: true } }),
      });

    renderOnboarding(OnboardingPage);

    // Navigate to split step
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));
    expect(screen.getByText("E/D/S Split")).toBeInTheDocument();

    // Skip split: should submit with defaults (50/30/20)
    fireEvent.click(screen.getByText("Skip (50/30/20)"));

    await waitFor(() => {
      expect(screen.getByText("Dashboard page")).toBeInTheDocument();
    });
  });

  it("navigates back from currency to welcome step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Welcome → Currency
    fireEvent.click(screen.getByText("Get started"));
    expect(screen.getByText("Default Currency")).toBeInTheDocument();

    // Back to Welcome
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Welcome to GoFin 🎉")).toBeInTheDocument();
  });

  it("navigates back from budget to currency step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Welcome → Currency → Budget
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    expect(screen.getByText("Monthly Budget")).toBeInTheDocument();

    // Back to Currency
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Default Currency")).toBeInTheDocument();
  });

  it("navigates back from split to budget step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Welcome → Currency → Budget → Split
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));
    expect(screen.getByText("E/D/S Split")).toBeInTheDocument();

    // Back to Budget
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Monthly Budget")).toBeInTheDocument();
  });

  it("selects a different currency", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    fireEvent.click(screen.getByText("Get started"));

    const currencySelect = screen.getByLabelText("Currency") as HTMLSelectElement;
    fireEvent.change(currencySelect, { target: { value: "EUR" } });
    expect(currencySelect.value).toBe("EUR");
  });

  it("enters a custom budget amount", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Navigate to budget step
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));

    const budgetInput = screen.getByLabelText("Budget Amount") as HTMLInputElement;
    fireEvent.change(budgetInput, { target: { value: "5000" } });
    expect(budgetInput.value).toBe("5000");
  });

  it("displays running total of E/D/S split", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Navigate to split step
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));

    // Default is 50+30+20=100
    expect(screen.getByText("Total: 100%")).toBeInTheDocument();

    // Change essentials
    const essentialsInput = screen.getByLabelText("Essentials %");
    fireEvent.change(essentialsInput, { target: { value: "60" } });
    expect(screen.getByText("Total: 110%")).toBeInTheDocument();
  });

  it("handles generic error from onboarding submission", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    // Network error (non-ApiRequestError)
    mockFetch.mockRejectedValueOnce(new TypeError("Network error"));

    renderOnboarding(OnboardingPage);

    // Navigate to split step and submit
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Complete Setup"));

    await waitFor(() => {
      expect(
        screen.getByText("An unexpected error occurred. Please try again."),
      ).toBeInTheDocument();
    });
  });

  it("clears split error when percentage input changes", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();
    renderOnboarding(OnboardingPage);

    // Navigate to split step
    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));

    // Make split invalid and submit
    const essentialsInput = screen.getByLabelText("Essentials %");
    fireEvent.change(essentialsInput, { target: { value: "40" } });
    fireEvent.click(screen.getByText("Complete Setup"));
    expect(screen.getByText(/must sum to 100%/)).toBeInTheDocument();

    // Changing input should clear the error
    fireEvent.change(essentialsInput, { target: { value: "50" } });
    expect(screen.queryByText(/must sum to 100%/)).not.toBeInTheDocument();
  });
});
