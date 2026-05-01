import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
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
      expect(screen.getByText("Settings")).toBeInTheDocument();
      expect(screen.getByText("test")).toBeInTheDocument(); // username
      expect(screen.getByText("Logout")).toBeInTheDocument();
      // Child route content rendered via Outlet
      expect(screen.getByText("Dashboard content")).toBeInTheDocument();
    });
  });
});
