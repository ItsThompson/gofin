import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { buildUser } from "@gofin/test-utils";
import { LandingPage } from "../LandingPage";
import { landingContent } from "../content";
import { setAuthStore, resetAuthMocks } from "./auth-mocks";

// The redirect is gone: both logged-out and logged-in visitors see the
// marketing page. The store is the boundary (mocked); MemoryRouter/Link stay
// real so CTA and nav hrefs are asserted against the rendered anchors.
vi.mock("@/stores/auth-store", () => ({
  useAuthStore: vi.fn(),
}));

function renderLandingPage() {
  return render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>,
  );
}

beforeEach(resetAuthMocks);

describe("LandingPage", () => {
  it("renders the marketing page with a Log in link for a logged-out visitor", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    renderLandingPage();

    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: landingContent.hero.heading }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: landingContent.login.label }),
    ).toHaveAttribute("href", "/login");
    expect(
      screen.queryByRole("button", { name: "Open account menu" }),
    ).not.toBeInTheDocument();

    // The hero and final CTAs share the "Get started" label; both point at
    // /register (the assembled page renders two of them).
    const getStartedLinks = screen.getAllByRole("link", {
      name: landingContent.hero.primaryCta.label,
    });
    expect(getStartedLinks).toHaveLength(2);
    for (const link of getStartedLinks) {
      expect(link).toHaveAttribute("href", "/register");
    }
  });

  it("keeps an authenticated visitor on the page and shows the avatar menu + Dashboard link", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user", username: "ada" }),
    });

    renderLandingPage();

    // No redirect: the marketing page is still mounted.
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: landingContent.hero.heading }),
    ).toBeInTheDocument();

    // Auth-aware header: avatar menu + Dashboard link, no Log in link.
    expect(
      screen.getByRole("button", { name: "Open account menu" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: landingContent.login.label }),
    ).not.toBeInTheDocument();
    const dashboardLinks = screen.getAllByRole("link", { name: "Dashboard" });
    expect(dashboardLinks[0]).toHaveAttribute("href", "/dashboard");
  });

  it("routes the Dashboard link to /admin for an authenticated admin", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin", username: "ops" }),
    });

    renderLandingPage();

    const dashboardLinks = screen.getAllByRole("link", { name: "Dashboard" });
    expect(dashboardLinks[0]).toHaveAttribute("href", "/admin");
  });

  it("assembles a single landmark set, one h1, and every section in order", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    renderLandingPage();

    // Exactly one of each page landmark and a single top-level heading.
    expect(screen.getAllByRole("banner")).toHaveLength(1); // <header>
    expect(screen.getAllByRole("main")).toHaveLength(1);
    expect(screen.getAllByRole("contentinfo")).toHaveLength(1); // <footer>
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);

    // Titled sections in document order; the value-prop band is headingless.
    const h2Text = screen
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent);
    expect(h2Text).toEqual([
      landingContent.howItWorks.heading,
      landingContent.threeWaySplit.heading,
      landingContent.featureShowcase.heading,
      landingContent.dualMode.heading,
      landingContent.faq.heading,
      landingContent.finalCta.heading,
    ]);

    // No heading skips past <h3>; the h3 count is the sum of the data-driven
    // section entries, guarding against a section dropping or duplicating cards.
    expect(screen.queryAllByRole("heading", { level: 4 })).toHaveLength(0);
    expect(screen.getAllByRole("heading", { level: 3 })).toHaveLength(
      landingContent.howItWorks.steps.length +
        landingContent.threeWaySplit.buckets.length +
        landingContent.featureShowcase.features.length +
        landingContent.dualMode.columns.length +
        landingContent.faq.items.length,
    );
  });

  it("renders no 'free' or 'without paying' copy anywhere on the page", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    const { container } = renderLandingPage();

    expect(container.textContent ?? "").not.toMatch(/\bfree\b/i);
    expect(container.textContent ?? "").not.toMatch(/without paying/i);
  });
});
