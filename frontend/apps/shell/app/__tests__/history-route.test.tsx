import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { useAuthStore } from "@/stores/auth-store";

// Mock the finance feature module so this test does not pull in its real tree
vi.mock("@gofin/finance/src/features/history", () => ({
  HistoryFeature: ({ user }: { user: { username: string } }) => (
    <div data-testid="history-feature">Budget History for {user.username}</div>
  ),
}));

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

async function importHistoryRoute() {
  const mod = await import("@/routes/history");
  return mod.default;
}

describe("HistoryRoute", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "No session" }),
    });
  });

  it("renders HistoryFeature with user when authenticated", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: true,
      user: authenticatedUser,
    });
    const HistoryRoute = await importHistoryRoute();

    const router = createMemoryRouter(
      [{ path: "/history", element: <HistoryRoute /> }],
      { initialEntries: ["/history"] },
    );
    render(<RouterProvider router={router} />);

    await waitFor(() => {
      expect(screen.getByTestId("history-feature")).toBeInTheDocument();
    });

    expect(
      screen.getByText("Budget History for testuser"),
    ).toBeInTheDocument();
  });

  it("renders nothing when user is null", async () => {
    resetStore({
      isLoading: false,
      isAuthenticated: false,
      user: null,
    });
    const HistoryRoute = await importHistoryRoute();

    const router = createMemoryRouter(
      [{ path: "/history", element: <HistoryRoute /> }],
      { initialEntries: ["/history"] },
    );
    const { container } = render(<RouterProvider router={router} />);

    // Route renders null when no user
    expect(container.querySelector("[data-testid='history-feature']")).toBeNull();
  });
});
