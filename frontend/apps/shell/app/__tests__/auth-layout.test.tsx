import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

const authenticatedUser = {
  id: "1",
  username: "testuser",
  email: "test@test.com",
  role: "user" as const,
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01",
};

const adminUser = {
  ...authenticatedUser,
  id: "admin-1",
  username: "admin",
  email: "admin@test.com",
  role: "admin" as const,
};

async function importAuthLayout() {
  const mod = await import("@/routes/auth-layout");
  return mod.default;
}

describe("AuthLayout - mobile menu and actions", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    // Default: checkAuth returns the authenticated user so the layout renders
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: authenticatedUser }),
    });
  });

  it("toggles mobile menu on hamburger click", async () => {
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
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    // Mobile hamburger button should be present
    const menuButton = screen.getByLabelText("Open menu");
    await userEvent.click(menuButton);

    // Mobile menu should show nav links
    // After click, the aria-label changes to "Close menu"
    expect(screen.getByLabelText("Close menu")).toBeInTheDocument();
  });

  it("closes mobile menu when a nav link is clicked", async () => {
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
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        {
          path: "/expenses",
          element: <AuthLayout />,
          children: [{ index: true, element: <div>Expenses content</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    // Open mobile menu
    await userEvent.click(screen.getByLabelText("Open menu"));

    // Click a nav link in the mobile menu (there are multiple "Expenses" links: desktop + mobile)
    const expenseLinks = screen.getAllByText("Expenses");
    // Click the last one (mobile menu link)
    await userEvent.click(expenseLinks[expenseLinks.length - 1]);

    // Mobile menu should close
    await waitFor(() => {
      expect(screen.getByLabelText("Open menu")).toBeInTheDocument();
    });
  });

  it("performs logout and navigates to /login", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      user: authenticatedUser,
    });
    const AuthLayout = await importAuthLayout();

    // First call is checkAuth (returns user), subsequent call is logout
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: authenticatedUser }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
      });

    const router = createMemoryRouter(
      [
        {
          path: "/dashboard",
          element: <AuthLayout />,
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        { path: "/login", element: <div>Login page</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    // Click logout button (desktop)
    await userEvent.click(screen.getByText("Logout"));

    await waitFor(() => {
      expect(screen.getByText("Login page")).toBeInTheDocument();
    });
  });

  it("shows admin nav link when user is admin", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      isAdmin: true,
      user: adminUser,
    });
    const AuthLayout = await importAuthLayout();

    const router = createMemoryRouter(
      [
        {
          path: "/dashboard",
          element: <AuthLayout />,
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    expect(screen.getByText("Admin")).toBeInTheDocument();
  });

  it("shows Return to Admin button when assuming identity", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      isAssuming: true,
      user: authenticatedUser,
    });
    const AuthLayout = await importAuthLayout();

    const router = createMemoryRouter(
      [
        {
          path: "/dashboard",
          element: <AuthLayout />,
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
        { path: "/admin", element: <div>Admin page</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    expect(screen.getByText("Return to Admin")).toBeInTheDocument();
  });

  it("restores identity and navigates to /admin on Return to Admin click", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      isAssuming: true,
      user: authenticatedUser,
    });
    const AuthLayout = await importAuthLayout();

    // checkAuth returns assumed user, then restoreIdentity returns admin
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: authenticatedUser }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: adminUser }),
      });

    const router = createMemoryRouter(
      [
        {
          path: "/dashboard",
          element: <AuthLayout />,
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
        { path: "/admin", element: <div>Admin page</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    await waitFor(() => {
      expect(screen.getByText("Return to Admin")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText("Return to Admin"));

    await waitFor(() => {
      expect(screen.getByText("Admin page")).toBeInTheDocument();
    });
  });

  it("does not show mobile FAB on /expenses/new route", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      user: authenticatedUser,
    });
    const AuthLayout = await importAuthLayout();

    const router = createMemoryRouter(
      [
        {
          path: "/expenses/new",
          element: <AuthLayout />,
          children: [{ index: true, element: <div>New expense form</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/expenses/new"] },
    );
    render(<RouterProvider router={router} />);

    // The "Log Expense" FAB should not appear on the new expense page itself
    expect(screen.queryByLabelText("Log Expense")).not.toBeInTheDocument();
  });

  it("shows mobile Log Expense FAB on other pages", async () => {
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
          children: [{ index: true, element: <div>Dashboard content</div> }],
        },
        { path: "/login", element: <div>Login redirect target</div> },
      ],
      { initialEntries: ["/dashboard"] },
    );
    render(<RouterProvider router={router} />);

    expect(screen.getByLabelText("Log Expense")).toBeInTheDocument();
  });
});
