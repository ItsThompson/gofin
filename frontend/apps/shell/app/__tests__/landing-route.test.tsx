import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { buildUser } from "@gofin/test-utils";
import { LandingPage, landingContent } from "@/features/marketing";
import { setAuthStore } from "@/features/marketing/__tests__/auth-mocks";

// The `/` route decision (US-ROUTE-01/02): an unauthenticated visitor keeps the
// marketing page; an authenticated visitor is redirected to getLandingPath(user)
// via useLandingRedirect. Unlike the LandingPage component test (which mocks
// useNavigate), this drives real navigation through a memory router so the
// redirect can be observed to leave no marketing mounted afterward. The auth
// store is the boundary (mocked); the router and Link stay real.
vi.mock("@/stores/auth-store", () => ({
  useAuthStore: vi.fn(),
}));

const DASHBOARD_CONTENT = "Dashboard route content";
const ADMIN_CONTENT = "Admin route content";
const LOGIN_CONTENT = "Login route content";

function renderAtRoot() {
  const router = createMemoryRouter(
    [
      { path: "/", element: <LandingPage /> },
      { path: "/dashboard", element: <div>{DASHBOARD_CONTENT}</div> },
      { path: "/admin", element: <div>{ADMIN_CONTENT}</div> },
      { path: "/login", element: <div>{LOGIN_CONTENT}</div> },
    ],
    { initialEntries: ["/"] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("/ route decision", () => {
  it("renders the full marketing page for an unauthenticated visitor and does not redirect to /login", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    const router = renderAtRoot();

    // The assembled marketing page renders at /: header, hero, every titled
    // section, and the footer landmark (exhaustive counts live in
    // LandingPage.test.tsx per §08).
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: landingContent.hero.heading,
      }),
    ).toBeInTheDocument();
    for (const heading of [
      landingContent.howItWorks.heading,
      landingContent.dualMode.heading,
      landingContent.faq.heading,
      landingContent.finalCta.heading,
    ]) {
      expect(
        screen.getByRole("heading", { level: 2, name: heading }),
      ).toBeInTheDocument();
    }
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();

    // Stayed on the public front door: no client redirect to /login.
    expect(router.state.location.pathname).toBe("/");
    expect(screen.queryByText(LOGIN_CONTENT)).not.toBeInTheDocument();
  });

  it("redirects an authenticated regular user to /dashboard and leaves no marketing mounted", async () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user" }),
    });

    const router = renderAtRoot();

    expect(await screen.findByText(DASHBOARD_CONTENT)).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/dashboard");
    expect(screen.queryByRole("main")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", {
        level: 1,
        name: landingContent.hero.heading,
      }),
    ).not.toBeInTheDocument();
  });

  it("redirects an authenticated admin to /admin and leaves no marketing mounted", async () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin" }),
    });

    const router = renderAtRoot();

    expect(await screen.findByText(ADMIN_CONTENT)).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/admin");
    expect(screen.queryByRole("main")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", {
        level: 1,
        name: landingContent.hero.heading,
      }),
    ).not.toBeInTheDocument();
  });
});
