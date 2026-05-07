import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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

async function importAuthLayout() {
  const mod = await import("@/routes/auth-layout");
  return mod.default;
}

describe("onboarding page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "No session" }),
    });
  });

  it("renders the welcome step initially", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    const router = createMemoryRouter(
      [{ path: "/onboarding", element: <OnboardingPage /> }],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    expect(screen.getByText("Welcome to GoFin 🎉")).toBeInTheDocument();
    expect(screen.getByText("Get started")).toBeInTheDocument();
    expect(screen.getByText("Step 1 of 4")).toBeInTheDocument();
  });

  it("advances from welcome to currency step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    const router = createMemoryRouter(
      [{ path: "/onboarding", element: <OnboardingPage /> }],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    fireEvent.click(screen.getByText("Get started"));

    expect(screen.getByText("Default Currency")).toBeInTheDocument();
    expect(screen.getByText("Step 2 of 4")).toBeInTheDocument();
  });

  it("advances through all steps to the split step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    const router = createMemoryRouter(
      [{ path: "/onboarding", element: <OnboardingPage /> }],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    fireEvent.click(screen.getByText("Get started"));
    expect(screen.getByText("Default Currency")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Continue"));
    expect(screen.getByText("Monthly Budget")).toBeInTheDocument();
    expect(screen.getByText("Step 3 of 4")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Continue"));
    expect(screen.getByText("E/D/S Split")).toBeInTheDocument();
    expect(screen.getByText("Step 4 of 4")).toBeInTheDocument();
  });

  it("validates E/D/S split sums to 100%", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    const router = createMemoryRouter(
      [{ path: "/onboarding", element: <OnboardingPage /> }],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));

    const essentialsInput = screen.getByLabelText("Essentials %");
    fireEvent.change(essentialsInput, { target: { value: "40" } });

    fireEvent.click(screen.getByText("Complete Setup"));

    expect(screen.getByText(/must sum to 100%/)).toBeInTheDocument();
  });

  it("allows E/D/S split that sums to exactly 100%", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

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

    const router = createMemoryRouter(
      [
        { path: "/onboarding", element: <OnboardingPage /> },
        { path: "/dashboard", element: <div>Dashboard page</div> },
      ],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));

    fireEvent.click(screen.getByText("Complete Setup"));

    await waitFor(() => {
      expect(screen.getByText("Dashboard page")).toBeInTheDocument();
    });
  });

  it("shows the skip option on the currency step", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    const router = createMemoryRouter(
      [{ path: "/onboarding", element: <OnboardingPage /> }],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    fireEvent.click(screen.getByText("Get started"));

    expect(screen.getByText("Skip (use USD)")).toBeInTheDocument();
  });

  it("shows error when finance API fails and allows retry", async () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: newUser });
    const OnboardingPage = await importOnboardingPage();

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message: "Database error" }),
    });

    const router = createMemoryRouter(
      [
        { path: "/onboarding", element: <OnboardingPage /> },
        { path: "/dashboard", element: <div>Dashboard page</div> },
      ],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    fireEvent.click(screen.getByText("Get started"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Continue"));
    fireEvent.click(screen.getByText("Complete Setup"));

    await waitFor(() => {
      expect(screen.getByText("Database error")).toBeInTheDocument();
    });

    expect(screen.getByText("Complete Setup")).toBeInTheDocument();
  });
});

describe("onboarding redirect guards", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "No session" }),
    });
  });

  it("redirects to /onboarding when user has not completed onboarding and navigates to /dashboard", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      user: newUser,
    });
    const AuthLayout = await importAuthLayout();

    const router = createMemoryRouter(
      [
        {
          path: "/dashboard",
          element: <AuthLayout />,
          children: [
            { index: true, element: <div>Dashboard content</div> },
          ],
        },
        { path: "/onboarding", element: <div>Onboarding page</div> },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    await waitFor(() => {
      expect(screen.getByText("Onboarding page")).toBeInTheDocument();
    });
  });

  it("redirects to /dashboard when completed user visits /onboarding", async () => {
    const onboardedUser = {
      id: "1",
      username: "test",
      email: "test@test.com",
      role: "user" as const,
      currency: "USD",
      hasCompletedOnboarding: true,
      createdAt: "2026-01-01",
    };

    resetStore({
      isLoading: false,
      isAuthenticated: true,
      user: onboardedUser,
    });
    const AuthLayout = await importAuthLayout();

    const router = createMemoryRouter(
      [
        {
          path: "/onboarding",
          element: <AuthLayout />,
          children: [
            { index: true, element: <div>Onboarding content</div> },
          ],
        },
        { path: "/dashboard", element: <div>Dashboard page</div> },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    await waitFor(() => {
      expect(screen.getByText("Dashboard page")).toBeInTheDocument();
    });
  });

  it("shows onboarding page without navbar for new user", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      user: newUser,
    });
    const AuthLayout = await importAuthLayout();
    const OnboardingPage = await importOnboardingPage();

    const router = createMemoryRouter(
      [
        {
          path: "/onboarding",
          element: <AuthLayout />,
          children: [
            { index: true, element: <OnboardingPage /> },
          ],
        },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/onboarding"] },
    );
    render(<RouterProvider router={router} />);

    expect(screen.getByText("Welcome to GoFin 🎉")).toBeInTheDocument();
    expect(screen.queryByText("Expenses")).not.toBeInTheDocument();
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
  });
});
