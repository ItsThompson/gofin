import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  createMemoryRouter,
  RouterProvider,
  type RouteObject,
} from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import type { RouteAccess } from "@/lib/route-access";

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

/**
 * Build a router whose auth-layout child carries a `handle.access`, mirroring
 * how the real route modules export their access metadata. useMatches() surfaces
 * the deepest handle, which is what the guard reads (Checkpoint 3).
 */
function layoutRoute(
  AuthLayout: React.ComponentType,
  path: string,
  access: RouteAccess,
  content: string,
): RouteObject {
  return {
    path,
    element: <AuthLayout />,
    children: [{ index: true, element: <div>{content}</div>, handle: { access } }],
  };
}

const destinationRoutes: RouteObject[] = [
  { path: "/login", element: <div>Login redirect target</div> },
];

function renderRouter(routes: RouteObject[], initialPath: string) {
  const router = createMemoryRouter(routes, { initialEntries: [initialPath] });
  return render(<RouterProvider router={router} />);
}

describe("AuthLayout - navbar, actions, and access guard", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: authenticatedUser }),
    });
  });

  it("toggles mobile menu on hamburger click", async () => {
    resetStore({ isAuthenticated: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    await userEvent.click(screen.getByLabelText("Open menu"));
    expect(screen.getByLabelText("Close menu")).toBeInTheDocument();
  });

  it("closes mobile menu when a nav link is clicked", async () => {
    resetStore({ isAuthenticated: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        layoutRoute(AuthLayout, "/expenses", "personal", "Expenses content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    await userEvent.click(screen.getByLabelText("Open menu"));

    const expenseLinks = screen.getAllByText("Expenses");
    await userEvent.click(expenseLinks[expenseLinks.length - 1]);

    await waitFor(() => {
      expect(screen.getByLabelText("Open menu")).toBeInTheDocument();
    });
  });

  it("performs logout and navigates to /login", async () => {
    resetStore({ isAuthenticated: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

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

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    await userEvent.click(screen.getByText("Logout"));

    await waitFor(() => {
      expect(screen.getByText("Login redirect target")).toBeInTheDocument();
    });
  });

  it("renders a 403 page for a direct admin on a personal route (no redirect, no finance UI flash)", async () => {
    resetStore({ isAuthenticated: true, isAdmin: true, user: adminUser });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: adminUser }),
    });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    expect(await screen.findByText("403: Access denied")).toBeInTheDocument();
    // No silent redirect to /admin and no flash of the finance page content.
    expect(screen.queryByText("Dashboard content")).not.toBeInTheDocument();
    expect(screen.queryByText("Admin page")).not.toBeInTheDocument();
    // Chrome is kept so the operator can navigate away.
    expect(screen.getByText("GoFin")).toBeInTheDocument();
  });

  it("renders a 403 page for a regular user on an admin route (symmetric guard)", async () => {
    resetStore({ isAuthenticated: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/admin", "admin", "Admin content"),
        ...destinationRoutes,
      ],
      "/admin",
    );

    expect(await screen.findByText("403: Access denied")).toBeInTheDocument();
    expect(screen.queryByText("Admin content")).not.toBeInTheDocument();
  });

  it("renders a 403 for a direct admin on /onboarding (personal), never the onboarding outlet", async () => {
    const unonboardedAdmin = { ...adminUser, hasCompletedOnboarding: false };
    resetStore({ isAuthenticated: true, isAdmin: true, user: unonboardedAdmin });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: unonboardedAdmin }),
    });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/onboarding", "personal", "Onboarding content"),
        ...destinationRoutes,
      ],
      "/onboarding",
    );

    expect(await screen.findByText("403: Access denied")).toBeInTheDocument();
    expect(screen.queryByText("Onboarding content")).not.toBeInTheDocument();
  });

  it("never routes an admin to onboarding: unonboarded admin renders the admin route", async () => {
    const unonboardedAdmin = { ...adminUser, hasCompletedOnboarding: false };
    resetStore({ isAuthenticated: true, isAdmin: true, user: unonboardedAdmin });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: unonboardedAdmin }),
    });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/admin", "admin", "Admin content"),
        ...destinationRoutes,
      ],
      "/admin",
    );

    expect(await screen.findByText("Admin content")).toBeInTheDocument();
    expect(screen.queryByText("Onboarding page")).not.toBeInTheDocument();
    expect(screen.queryByText("403: Access denied")).not.toBeInTheDocument();
  });

  it("lets an assumed session (role=user) pass a personal route and shows Return to Admin", async () => {
    resetStore({ isAuthenticated: true, isAssuming: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    expect(await screen.findByText("Dashboard content")).toBeInTheDocument();
    expect(screen.getByText("Return to Admin")).toBeInTheDocument();
    expect(screen.queryByText("403: Access denied")).not.toBeInTheDocument();
  });

  it("shows only Admin and Settings nav for a direct admin (no finance links, no FAB)", async () => {
    resetStore({ isAuthenticated: true, isAdmin: true, user: adminUser });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: adminUser }),
    });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/settings", "authenticated", "Settings content"),
        ...destinationRoutes,
      ],
      "/settings",
    );

    expect(await screen.findByText("Admin")).toBeInTheDocument();
    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
    expect(screen.queryByText("Expenses")).not.toBeInTheDocument();
    expect(screen.queryByText("History")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Log Expense")).not.toBeInTheDocument();

    expect(screen.getByText("GoFin").closest("a")).toHaveAttribute(
      "href",
      "/admin",
    );
  });

  it("keeps full user nav plus Return to Admin for an assumed session", async () => {
    resetStore({ isAuthenticated: true, isAssuming: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    expect(await screen.findByText("Dashboard")).toBeInTheDocument();
    expect(screen.getByText("Expenses")).toBeInTheDocument();
    expect(screen.getByText("History")).toBeInTheDocument();
    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.getByText("Return to Admin")).toBeInTheDocument();
    expect(screen.queryByText("Admin")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Log Expense")).not.toBeInTheDocument();
    expect(screen.getByText("GoFin").closest("a")).toHaveAttribute(
      "href",
      "/dashboard",
    );
  });

  it("restores identity and navigates to /admin on Return to Admin click", async () => {
    resetStore({ isAuthenticated: true, isAssuming: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

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

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
        { path: "/admin", element: <div>Admin page</div> },
      ],
      "/dashboard",
    );

    await waitFor(() => {
      expect(screen.getByText("Return to Admin")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText("Return to Admin"));

    await waitFor(() => {
      expect(screen.getByText("Admin page")).toBeInTheDocument();
    });
  });

  it("does not show mobile FAB on /expenses/new route", async () => {
    resetStore({ isAuthenticated: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/expenses/new", "personal", "New expense form"),
        ...destinationRoutes,
      ],
      "/expenses/new",
    );

    expect(await screen.findByText("New expense form")).toBeInTheDocument();
    expect(screen.queryByLabelText("Log Expense")).not.toBeInTheDocument();
  });

  it("shows mobile Log Expense FAB on other pages", async () => {
    resetStore({ isAuthenticated: true, user: authenticatedUser });
    const AuthLayout = await importAuthLayout();

    renderRouter(
      [
        layoutRoute(AuthLayout, "/dashboard", "personal", "Dashboard content"),
        ...destinationRoutes,
      ],
      "/dashboard",
    );

    expect(await screen.findByLabelText("Log Expense")).toBeInTheDocument();
  });
});
