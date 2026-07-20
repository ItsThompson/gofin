import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { buildUser } from "@gofin/test-utils";
import { LandingPage, landingContent } from "@/features/marketing";
import { setAuthStore } from "@/features/marketing/__tests__/auth-mocks";

// The `/` route serves the marketing page to both unauthenticated and
// authenticated visitors. Unlike the LandingPage
// component test, this drives the page through a real memory router so we can
// assert the URL stays at `/` and no other route mounts. The auth store is the
// boundary (mocked); the router and Link stay real.
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
  it("renders the full marketing page for an unauthenticated visitor", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    const router = renderAtRoot();

    // The assembled marketing page renders at /: header, hero, every titled
    // section, and the footer landmark (exhaustive counts live in
    // LandingPage.test.tsx).
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: landingContent.hero.heading,
      }),
    ).toBeInTheDocument();
    for (const heading of [
      landingContent.howItWorks.heading,
      landingContent.threeWaySplit.heading,
      landingContent.featureShowcase.heading,
      landingContent.dualMode.heading,
      landingContent.faq.heading,
      landingContent.finalCta.heading,
    ]) {
      expect(
        screen.getByRole("heading", { level: 2, name: heading }),
      ).toBeInTheDocument();
    }
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();

    // Stayed on the public front door; the Log in link is shown, not consumed.
    expect(router.state.location.pathname).toBe("/");
    expect(screen.queryByText(LOGIN_CONTENT)).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: landingContent.login.label }),
    ).toHaveAttribute("href", "/login");
  });

  it("keeps an authenticated regular user on the marketing page (no redirect)", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user", username: "ada" }),
    });

    const router = renderAtRoot();

    expect(router.state.location.pathname).toBe("/");
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.queryByText(DASHBOARD_CONTENT)).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Open account menu" }),
    ).toBeInTheDocument();
  });

  it("keeps an authenticated admin on the marketing page with a role-aware Dashboard link", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin", username: "ops" }),
    });

    const router = renderAtRoot();

    expect(router.state.location.pathname).toBe("/");
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.queryByText(ADMIN_CONTENT)).not.toBeInTheDocument();

    const dashboardLinks = screen.getAllByRole("link", { name: "Dashboard" });
    expect(dashboardLinks[0]).toHaveAttribute("href", "/admin");
  });
});
