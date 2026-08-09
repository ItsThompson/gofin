import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  createMemoryRouter,
  RouterProvider,
  type RouteObject,
} from "react-router";

/** The subset of Sentry's CaptureContext that reportError sends. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException, toastError } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
  toastError: vi.fn(),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));
vi.mock("sonner", () => ({
  toast: { error: toastError, success: vi.fn() },
  Toaster: () => null,
}));

import { useAuthStore } from "@/stores/auth-store";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const assumedUser = {
  id: "1",
  username: "testuser",
  email: "test@test.com",
  role: "user" as const,
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01",
};

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
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

function errorResponse(status: number) {
  return {
    ok: false,
    status,
    json: () => Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message: "boom" }),
  };
}

beforeEach(() => {
  captureException.mockClear();
  toastError.mockClear();
  mockFetch.mockReset();
  resetStore();
});

describe("checkAuth when the check never completes", () => {
  it("reports the outage it keeps the session through", async () => {
    mockFetch.mockResolvedValueOnce(errorResponse(500));
    resetStore({ isAuthenticated: true, user: assumedUser });

    await useAuthStore.getState().checkAuth();

    expect(useAuthStore.getState()).toMatchObject({
      isAuthenticated: true,
      authError: "unavailable",
    });
    expect(onlyCapture().context.tags).toMatchObject({
      error_kind: "upstream",
      operation: "auth.check",
      domain: "auth",
    });
  });

  it("classifies a dropped connection as a network failure", async () => {
    mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));

    await useAuthStore.getState().checkAuth();

    const { context } = onlyCapture();
    expect(context.tags).toMatchObject({ error_kind: "network", operation: "auth.check" });
    expect(context.level).toBe("warning");
  });

  it("reports nothing for a 401, which is the no-session answer", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "no session" }),
    });

    await useAuthStore.getState().checkAuth();

    expect(useAuthStore.getState().authError).toBe("unauthorized");
    expect(captureException).not.toHaveBeenCalled();
  });

  it("reports nothing when the check succeeds", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: assumedUser }),
    });

    await useAuthStore.getState().checkAuth();

    expect(captureException).not.toHaveBeenCalled();
  });
});

describe("a failed return to the admin identity", () => {
  async function renderAssumedLayout() {
    const AuthLayout = (await import("@/routes/auth-layout")).default;
    const routes: RouteObject[] = [
      {
        path: "/dashboard",
        element: <AuthLayout />,
        children: [
          {
            index: true,
            element: <div>Dashboard content</div>,
            handle: { access: "personal" },
          },
        ],
      },
      { path: "/login", element: <div>Login redirect target</div> },
      { path: "/admin", element: <div>Admin page</div> },
    ];
    const router = createMemoryRouter(routes, {
      initialEntries: ["/dashboard"],
    });
    render(<RouterProvider router={router} />);
    await waitFor(() =>
      expect(screen.getByText("Return to Admin")).toBeInTheDocument(),
    );
  }

  it("reports, tells the admin, and leaves the button usable", async () => {
    resetStore({ isAuthenticated: true, isAssuming: true, user: assumedUser });
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: assumedUser }),
      })
      .mockResolvedValueOnce(errorResponse(503));

    await renderAssumedLayout();
    await userEvent.click(screen.getByText("Return to Admin"));

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    expect(onlyCapture().context.tags).toMatchObject({
      error_kind: "upstream",
      operation: "auth.restore_identity",
      domain: "auth",
    });
    expect(toastError).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Return to Admin")).toBeEnabled();
    expect(screen.queryByText("Admin page")).toBeNull();
  });
});
