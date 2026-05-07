import { describe, it, expect, beforeEach, vi } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { buildUser, createMockApi, renderWithRouter } from "@gofin/test-utils";
import { useAuthStore } from "@/stores/auth-store";

// Import through the route (thin wrapper) to test the full integration
async function importRegisterPage() {
  const mod = await import("@/routes/register");
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

async function renderRegisterPage() {
  const RegisterPage = await importRegisterPage();

  return renderWithRouter(<RegisterPage />, {
    route: "/register",
    routeConfig: [
      { path: "/register", element: <RegisterPage /> },
      { path: "/dashboard", element: <div>Dashboard page</div> },
      { path: "/onboarding", element: <div>Onboarding page</div> },
    ],
  });
}

function submitForm() {
  const form = screen.getByRole("button", { name: "Create account" }).closest("form")!;
  fireEvent.submit(form);
}

describe("register page", () => {
  beforeEach(() => {
    resetStore({ isLoading: false, isAuthenticated: false });
    vi.restoreAllMocks();
  });

  describe("form validation", () => {
    it("shows 'Username is required' when submitting with empty username", async () => {
      setupUnauthenticatedMock();
      await renderRegisterPage();

      submitForm();

      await waitFor(() => {
        expect(screen.getByText("Username is required")).toBeInTheDocument();
      });
    });

    it("shows 'Username must be at least 2 characters' for short username", async () => {
      setupUnauthenticatedMock();
      await renderRegisterPage();

      fireEvent.change(screen.getByLabelText("Username"), { target: { value: "a" } });
      submitForm();

      await waitFor(() => {
        expect(screen.getByText("Username must be at least 2 characters")).toBeInTheDocument();
      });
    });

    it("shows password strength error for weak password", async () => {
      setupUnauthenticatedMock();
      await renderRegisterPage();

      fireEvent.change(screen.getByLabelText("Username"), { target: { value: "validuser" } });
      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "weak" } });
      submitForm();

      await waitFor(() => {
        expect(
          screen.getByText(
            "Password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit",
          ),
        ).toBeInTheDocument();
      });
    });
  });

  describe("API error handling", () => {
    it("displays server message when username is already taken", async () => {
      setupUnauthenticatedMock({
        "/api/auth/register": {
          status: 409,
          body: { code: "DUPLICATE_USERNAME", message: "Username is already taken" },
        },
      });
      await renderRegisterPage();

      fireEvent.change(screen.getByLabelText("Username"), { target: { value: "taken" } });
      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "user@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password1" } });
      submitForm();

      await waitFor(() => {
        expect(screen.getByText("Username is already taken")).toBeInTheDocument();
      });

      // Must NOT navigate to onboarding on field-specific errors
      expect(screen.queryByText("Onboarding page")).not.toBeInTheDocument();
    });

    it("does not navigate when email is already taken", async () => {
      setupUnauthenticatedMock({
        "/api/auth/register": {
          status: 409,
          body: { code: "DUPLICATE_EMAIL", message: "Email is already in use" },
        },
      });
      await renderRegisterPage();

      fireEvent.change(screen.getByLabelText("Username"), { target: { value: "newuser" } });
      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "taken@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password1" } });
      submitForm();

      await waitFor(() => {
        expect(screen.getByText("Email is already in use")).toBeInTheDocument();
      });

      // Must NOT navigate to onboarding on field-specific errors
      expect(screen.queryByText("Onboarding page")).not.toBeInTheDocument();
    });
  });

  describe("successful registration", () => {
    it("auto-logs in and redirects to /onboarding", async () => {
      const user = buildUser({ hasCompletedOnboarding: false });
      setupUnauthenticatedMock({
        "/api/auth/register": { user },
      });
      await renderRegisterPage();

      fireEvent.change(screen.getByLabelText("Username"), { target: { value: "newuser" } });
      fireEvent.change(screen.getByLabelText("Email"), { target: { value: "new@example.com" } });
      fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password1" } });
      submitForm();

      await waitFor(() => {
        expect(screen.getByText("Onboarding page")).toBeInTheDocument();
      });

      // Verify user is now in the auth store (auto-logged in)
      const state = useAuthStore.getState();
      expect(state.isAuthenticated).toBe(true);
      expect(state.user?.id).toBe(user.id);
    });
  });
});
