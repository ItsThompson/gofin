import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { useAuthStore } from "@/stores/auth-store";

// Mock fetch globally to prevent real network calls
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
    authError: null,
    ...overrides,
  });
}

const authenticatedUser = {
  id: "1",
  username: "test",
  email: "test@test.com",
  role: "user" as const,
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01",
};

// Lazy-import the real route modules so their side effects don't fire at import time
async function importLoginPage() {
  const mod = await import("@/routes/login");
  return mod.default;
}

async function importAuthLayout() {
  const mod = await import("@/routes/auth-layout");
  return mod.default;
}

function renderRoute(path: string, element: React.ReactNode) {
  // A catch-all route ensures we can observe navigations (redirects change the URL,
  // and we see whatever the router resolves).
  const router = createMemoryRouter(
    [
      { path, element },
      { path: "/dashboard", element: <div>Dashboard page</div> },
      { path: "/login", element: <div>Login redirect target</div> },
    ],
    { initialEntries: [path] },
  );
  return render(<RouterProvider router={router} />);
}

describe("auth guard redirect logic", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    // Default: prevent checkAuth from firing real requests
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "No session" }),
    });
  });

  describe("login page (unauthenticated route guard)", () => {
    it("shows login form when not authenticated", async () => {
      resetStore({ isLoading: false, isAuthenticated: false });
      const LoginPage = await importLoginPage();

      renderRoute("/login", <LoginPage />);

      expect(screen.getByText("Sign in to GoFin")).toBeInTheDocument();
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
      expect(screen.getByLabelText("Password")).toBeInTheDocument();
    });

    it("redirects to dashboard when already authenticated", async () => {
      resetStore({
        isLoading: false,
        isAuthenticated: true,
        user: authenticatedUser,
      });
      const LoginPage = await importLoginPage();

      renderRoute("/login", <LoginPage />);

      // Should navigate away from login: the dashboard route renders instead
      expect(screen.getByText("Dashboard page")).toBeInTheDocument();
    });

    it("shows loading state while checking auth", async () => {
      resetStore({ isLoading: true });
      const LoginPage = await importLoginPage();

      renderRoute("/login", <LoginPage />);

      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });
  });

  describe("auth layout (authenticated route guard)", () => {
    it("redirects to login when not authenticated", async () => {
      resetStore({ isLoading: false, isAuthenticated: false });
      const AuthLayout = await importAuthLayout();

      renderRoute(
        "/dashboard",
        <AuthLayout />,
      );

      // Should navigate to /login
      expect(screen.getByText("Login redirect target")).toBeInTheDocument();
    });

    it("shows the unreachable-backend screen instead of bouncing to login", async () => {
      // The layout's own checkAuth runs on mount: a 500 must leave the user on
      // this screen rather than at /login.
      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () =>
          Promise.resolve({
            code: "INTERNAL_SERVER_ERROR",
            message: "Server error",
          }),
      });
      resetStore({ isLoading: true, isAuthenticated: false });
      const AuthLayout = await importAuthLayout();

      renderRoute("/dashboard", <AuthLayout />);

      await waitFor(() => {
        expect(screen.getByText("GoFin is unreachable")).toBeInTheDocument();
      });
      expect(
        screen.queryByText("Login redirect target"),
      ).not.toBeInTheDocument();
    });

    it("retries the auth check from the unreachable-backend screen", async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () =>
          Promise.resolve({
            code: "INTERNAL_SERVER_ERROR",
            message: "Server error",
          }),
      });
      resetStore({ isLoading: true, isAuthenticated: false });
      const AuthLayout = await importAuthLayout();

      renderRoute("/dashboard", <AuthLayout />);

      await waitFor(() => {
        expect(screen.getByText("GoFin is unreachable")).toBeInTheDocument();
      });

      mockFetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: authenticatedUser }),
      });
      await userEvent.click(screen.getByRole("button", { name: "Try again" }));

      await waitFor(() => {
        expect(useAuthStore.getState().isAuthenticated).toBe(true);
      });
      expect(
        screen.queryByText("GoFin is unreachable"),
      ).not.toBeInTheDocument();
    });

    it("re-enables the retry when the backend is still unreachable", async () => {
      const serverError = {
        ok: false,
        status: 500,
        json: () =>
          Promise.resolve({
            code: "INTERNAL_SERVER_ERROR",
            message: "Server error",
          }),
      };
      mockFetch.mockResolvedValue(serverError);
      resetStore({ isLoading: true, isAuthenticated: false });
      const AuthLayout = await importAuthLayout();

      renderRoute("/dashboard", <AuthLayout />);

      await waitFor(() => {
        expect(screen.getByText("GoFin is unreachable")).toBeInTheDocument();
      });

      // The state a user in a real outage hits most: retry, still down, retry.
      await userEvent.click(screen.getByRole("button", { name: "Try again" }));

      const retryButton = await screen.findByRole("button", {
        name: "Try again",
      });
      expect(retryButton).toBeEnabled();
      expect(screen.getByText("GoFin is unreachable")).toBeInTheDocument();
      expect(useAuthStore.getState().authError).toBe("unavailable");
    });

    it("shows the retry as pending while the auth check is in flight", async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () =>
          Promise.resolve({
            code: "INTERNAL_SERVER_ERROR",
            message: "Server error",
          }),
      });
      resetStore({ isLoading: true, isAuthenticated: false });
      const AuthLayout = await importAuthLayout();

      renderRoute("/dashboard", <AuthLayout />);

      await waitFor(() => {
        expect(screen.getByText("GoFin is unreachable")).toBeInTheDocument();
      });

      // A hanging backend is the state the user is already in: the button must
      // not look inert for the whole gateway timeout.
      let releaseRetry!: (value: unknown) => void;
      mockFetch.mockReturnValue(
        new Promise((resolve) => {
          releaseRetry = resolve;
        }),
      );
      await userEvent.click(screen.getByRole("button", { name: "Try again" }));

      const pendingButton = await screen.findByRole("button", {
        name: "Retrying...",
      });
      expect(pendingButton).toBeDisabled();

      await act(async () => {
        releaseRetry({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ user: authenticatedUser }),
        });
      });

      await waitFor(() => {
        expect(useAuthStore.getState().isAuthenticated).toBe(true);
      });
    });

    it("shows loading state while checking auth", async () => {
      resetStore({ isLoading: true });
      const AuthLayout = await importAuthLayout();

      renderRoute("/dashboard", <AuthLayout />);

      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });

    it("renders navbar when authenticated", async () => {
      resetStore({
        isLoading: false,
        isAuthenticated: true,
        user: authenticatedUser,
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
          { path: "/login", element: <div>Login redirect target</div> },
        ],
        { initialEntries: ["/dashboard"] },
      );
      render(<RouterProvider router={router} />);

      // Navbar elements
      expect(screen.getByText("GoFin")).toBeInTheDocument();
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
      expect(screen.getByText("Expenses")).toBeInTheDocument();
      expect(screen.getByText("History")).toBeInTheDocument();
      expect(screen.getByText("Settings")).toBeInTheDocument();
      expect(screen.getByText("test")).toBeInTheDocument(); // username
      expect(screen.getByText("Logout")).toBeInTheDocument();
      // Child route content rendered via Outlet
      expect(screen.getByText("Dashboard content")).toBeInTheDocument();
    });

    it("renders History nav link between Expenses and Settings", async () => {
      resetStore({
        isLoading: false,
        isAuthenticated: true,
        user: authenticatedUser,
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
          { path: "/login", element: <div>Login redirect target</div> },
        ],
        { initialEntries: ["/dashboard"] },
      );
      render(<RouterProvider router={router} />);

      // Verify History link exists and points to /history
      const historyLinks = screen.getAllByText("History");
      expect(historyLinks.length).toBeGreaterThan(0);

      const historyLink = historyLinks[0].closest("a");
      expect(historyLink).toHaveAttribute("href", "/history");
    });

    it("shows active state on History nav link when on /history", async () => {
      resetStore({
        isLoading: false,
        isAuthenticated: true,
        user: authenticatedUser,
      });
      const AuthLayout = await importAuthLayout();

      const router = createMemoryRouter(
        [
          {
            path: "/history",
            element: <AuthLayout />,
            children: [
              { index: true, element: <div>History content</div> },
            ],
          },
          { path: "/login", element: <div>Login redirect target</div> },
        ],
        { initialEntries: ["/history"] },
      );
      render(<RouterProvider router={router} />);

      // The active nav link gets the "bg-muted text-foreground" class
      const historyLinks = screen.getAllByText("History");
      const desktopLink = historyLinks[0].closest("a");
      expect(desktopLink).toHaveClass("bg-muted");
      expect(desktopLink).toHaveClass("text-foreground");
    });

    it("redirects to /login when accessing /history unauthenticated", async () => {
      resetStore({ isLoading: false, isAuthenticated: false });
      const AuthLayout = await importAuthLayout();

      const router = createMemoryRouter(
        [
          {
            path: "/history",
            element: <AuthLayout />,
            children: [
              { index: true, element: <div>History content</div> },
            ],
          },
          { path: "/login", element: <div>Login redirect target</div> },
        ],
        { initialEntries: ["/history"] },
      );
      render(<RouterProvider router={router} />);

      expect(screen.getByText("Login redirect target")).toBeInTheDocument();
    });
  });
});
