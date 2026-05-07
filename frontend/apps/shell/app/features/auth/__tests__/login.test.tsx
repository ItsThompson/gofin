import { describe, it, expect, beforeEach, vi } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { buildUser, createMockApi, renderWithRouter } from "@gofin/test-utils";
import { useAuthStore } from "@/stores/auth-store";

// Import through the route (thin wrapper) to test the full integration
async function importLoginPage() {
  const mod = await import("@/routes/login");
  return mod.default;
}

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

function setupUnauthenticatedMock(extraRoutes: Record<string, unknown> = {}) {
  const mockFetch = createMockApi({
    "/api/auth/me": { status: 401, body: { code: "UNAUTHORIZED", message: "Not authenticated" } },
    ...extraRoutes,
  });
  global.fetch = mockFetch;
  return mockFetch;
}

async function renderLoginPage(options?: { route?: string; searchParams?: Record<string, string> }) {
  const LoginPage = await importLoginPage();
  const route = options?.route ?? "/login";

  return renderWithRouter(<LoginPage />, {
    route,
    searchParams: options?.searchParams,
    routeConfig: [
      { path: "/login", element: <LoginPage /> },
      { path: "/dashboard", element: <div>Dashboard page</div> },
      { path: "/onboarding", element: <div>Onboarding page</div> },
      { path: "/expenses", element: <div>Expenses page</div> },
    ],
  });
}

describe("login page", () => {
  beforeEach(() => {
    resetStore({ isLoading: false, isAuthenticated: false });
    vi.restoreAllMocks();
  });

  describe("form validation", () => {
    it("shows 'Email is required' when submitting with empty email", async () => {
      setupUnauthenticatedMock();
      await renderLoginPage();

      const form = screen.getByRole("button", { name: "Sign in" }).closest("form")!;
      fireEvent.submit(form);

      await waitFor(() => {
        expect(screen.getByText("Email is required")).toBeInTheDocument();
      });
    });

    it("shows 'Please enter a valid email address' for invalid email format", async () => {
      setupUnauthenticatedMock();
      await renderLoginPage();

      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "not-an-email" } });
      const form = screen.getByRole("button", { name: "Sign in" }).closest("form")!;
      fireEvent.submit(form);

      await waitFor(() => {
        expect(screen.getByText("Please enter a valid email address")).toBeInTheDocument();
      });
    });

    it("shows 'Password is required' when submitting with valid email but empty password", async () => {
      setupUnauthenticatedMock();
      await renderLoginPage();

      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      const form = screen.getByRole("button", { name: "Sign in" }).closest("form")!;
      fireEvent.submit(form);

      await waitFor(() => {
        expect(screen.getByText("Password is required")).toBeInTheDocument();
      });
    });
  });

  describe("API error handling", () => {
    it("displays the server error message on wrong credentials", async () => {
      setupUnauthenticatedMock({
        "/api/auth/login": { status: 401, body: { code: "INVALID_CREDENTIALS", message: "Invalid email or password" } },
      });
      await renderLoginPage();

      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "WrongPass1" } });
      fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

      await waitFor(() => {
        expect(screen.getByText("Invalid email or password")).toBeInTheDocument();
      });
    });
  });

  describe("successful login", () => {
    it("redirects to /dashboard after login", async () => {
      const user = buildUser({ hasCompletedOnboarding: true });
      setupUnauthenticatedMock({
        "/api/auth/login": { user },
      });
      await renderLoginPage();

      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password1" } });
      fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

      await waitFor(() => {
        expect(screen.getByText("Dashboard page")).toBeInTheDocument();
      });
    });

    it("redirects to /onboarding for non-onboarded user", async () => {
      const user = buildUser({ hasCompletedOnboarding: false });
      setupUnauthenticatedMock({
        "/api/auth/login": { user },
      });
      await renderLoginPage();

      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password1" } });
      fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

      await waitFor(() => {
        expect(screen.getByText("Onboarding page")).toBeInTheDocument();
      });
    });
  });

  describe("session expiry", () => {
    it("shows 'Your session has expired' banner when navigating to /login?expired=true", async () => {
      setupUnauthenticatedMock();
      await renderLoginPage({ searchParams: { expired: "true" } });

      await waitFor(() => {
        expect(screen.getByText(/Your session has expired/)).toBeInTheDocument();
      });
    });
  });

  describe("consumeReturnToPath redirect", () => {
    it("redirects to saved path from sessionStorage after login", async () => {
      const user = buildUser({ hasCompletedOnboarding: true });
      setupUnauthenticatedMock({
        "/api/auth/login": { user },
      });

      vi.spyOn(Storage.prototype, "getItem").mockImplementation((key: string) => {
        if (key === "gofin_return_to") return "/expenses";
        return null;
      });
      vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {});

      await renderLoginPage();

      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password1" } });
      fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

      await waitFor(() => {
        expect(screen.getByText("Expenses page")).toBeInTheDocument();
      });
    });
  });

  describe("already-authenticated redirect", () => {
    it("redirects to /dashboard when user is already authenticated", async () => {
      const user = buildUser({ hasCompletedOnboarding: true });
      resetStore({ isLoading: false, isAuthenticated: true, user });

      const mockFetch = createMockApi({
        "/api/auth/me": { user },
      });
      global.fetch = mockFetch;

      await renderLoginPage();

      await waitFor(() => {
        expect(screen.getByText("Dashboard page")).toBeInTheDocument();
      });
    });
  });
});
