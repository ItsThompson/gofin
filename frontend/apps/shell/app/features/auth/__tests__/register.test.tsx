import { describe, it, expect, beforeEach, vi } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
    authError: null,
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

function submitButton() {
  return screen.getByRole("button", { name: "Create account" });
}

/**
 * Fill every required field with valid values so a real submit click clears the
 * inputs' native required/type=email constraints and reaches the submit handler.
 * Callers override individual fields to exercise a specific field-level error.
 */
async function fillValidForm(
  user: ReturnType<typeof userEvent.setup>,
  overrides: Partial<Record<"username" | "email" | "password", string>> = {},
) {
  const values = {
    username: "newuser",
    email: "new@example.com",
    password: "Password1",
    ...overrides,
  };
  await user.type(screen.getByLabelText("Username"), values.username);
  await user.type(screen.getByLabelText("Email"), values.email);
  await user.type(screen.getByLabelText("Password"), values.password);
  await user.type(screen.getByLabelText("Confirm Password"), values.password);
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

      // Submitted empty to exercise the handler's own guard; a real click would
      // be blocked by the inputs' native required constraints first.
      const form = submitButton().closest("form")!;
      fireEvent.submit(form);

      await waitFor(() => {
        expect(screen.getByText("Username is required")).toBeInTheDocument();
      });
    });

    it("shows 'Username must be at least 2 characters' for short username", async () => {
      const user = userEvent.setup();
      setupUnauthenticatedMock();
      await renderRegisterPage();

      await fillValidForm(user, { username: "a" });
      await user.click(submitButton());

      await waitFor(() => {
        expect(screen.getByText("Username must be at least 2 characters")).toBeInTheDocument();
      });
    });

    it("shows password strength error for weak password", async () => {
      const user = userEvent.setup();
      setupUnauthenticatedMock();
      await renderRegisterPage();

      await fillValidForm(user, { password: "weak" });
      await user.click(submitButton());

      await waitFor(() => {
        expect(
          screen.getByText("Password must be at least 8 characters"),
        ).toBeInTheDocument();
      });
    });
  });

  describe("API error handling", () => {
    it("displays server message when username is already taken", async () => {
      const user = userEvent.setup();
      setupUnauthenticatedMock({
        "/api/auth/register": {
          status: 409,
          body: { code: "DUPLICATE_USERNAME", message: "Username is already taken" },
        },
      });
      await renderRegisterPage();

      await fillValidForm(user, { username: "taken" });
      await user.click(submitButton());

      await waitFor(() => {
        expect(screen.getByText("Username is already taken")).toBeInTheDocument();
      });

      // Must NOT navigate to onboarding on field-specific errors
      expect(screen.queryByText("Onboarding page")).not.toBeInTheDocument();
    });

    it("does not navigate when email is already taken", async () => {
      const user = userEvent.setup();
      setupUnauthenticatedMock({
        "/api/auth/register": {
          status: 409,
          body: { code: "DUPLICATE_EMAIL", message: "Email is already in use" },
        },
      });
      await renderRegisterPage();

      await fillValidForm(user, { email: "taken@example.com" });
      await user.click(submitButton());

      await waitFor(() => {
        expect(screen.getByText("Email is already in use")).toBeInTheDocument();
      });

      // Must NOT navigate to onboarding on field-specific errors
      expect(screen.queryByText("Onboarding page")).not.toBeInTheDocument();
    });
  });

  describe("successful registration", () => {
    it("auto-logs in and redirects to /onboarding", async () => {
      const user = userEvent.setup();
      const newUser = buildUser({ hasCompletedOnboarding: false });
      setupUnauthenticatedMock({
        "/api/auth/register": { user: newUser },
      });
      await renderRegisterPage();

      await fillValidForm(user);
      await user.click(submitButton());

      await waitFor(() => {
        expect(screen.getByText("Onboarding page")).toBeInTheDocument();
      });

      // Verify user is now in the auth store (auto-logged in)
      const state = useAuthStore.getState();
      expect(state.isAuthenticated).toBe(true);
      expect(state.user?.id).toBe(newUser.id);
    });
  });
});
