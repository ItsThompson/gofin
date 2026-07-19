import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { buildUser } from "@gofin/test-utils";
import { LandingPage } from "../LandingPage";
import { landingContent } from "../content";
import { mockNavigate, setAuthStore, resetAuthMocks } from "./auth-mocks";

// The `/` route decision: an unauthenticated visitor keeps the marketing page;
// an authenticated visitor is redirected via getLandingPath. The store and
// useNavigate are the boundaries; MemoryRouter/Link stay real so CTA hrefs are
// asserted against the rendered anchors.
vi.mock("react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-router")>()),
  useNavigate: vi.fn(),
}));

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
  it("renders the marketing page and does not redirect an unauthenticated visitor", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    renderLandingPage();

    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: landingContent.hero.heading }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: landingContent.login.label }),
    ).toHaveAttribute("href", "/login");

    // The hero and final CTAs share the "Get started" label; both must point at
    // /register (the assembled page renders two of them).
    const getStartedLinks = screen.getAllByRole("link", {
      name: landingContent.hero.primaryCta.label,
    });
    expect(getStartedLinks).toHaveLength(2);
    for (const link of getStartedLinks) {
      expect(link).toHaveAttribute("href", "/register");
    }

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("assembles a single landmark set, one h1, and every section's content", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    renderLandingPage();

    // Exactly one of each page landmark and a single top-level heading.
    expect(screen.getAllByRole("banner")).toHaveLength(1); // <header>
    expect(screen.getAllByRole("main")).toHaveLength(1);
    expect(screen.getAllByRole("contentinfo")).toHaveLength(1); // <footer>
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);

    // How-it-works renders one <h3> card per content step.
    for (const step of landingContent.howItWorks.steps) {
      expect(
        screen.getByRole("heading", { level: 3, name: step.title }),
      ).toBeInTheDocument();
    }

    // Dual-mode renders exactly two columns, one <h3> each.
    expect(landingContent.dualMode.columns).toHaveLength(2);
    for (const column of landingContent.dualMode.columns) {
      expect(
        screen.getByRole("heading", { level: 3, name: column.title }),
      ).toBeInTheDocument();
    }

    // FAQ renders one <h3> entry per content item.
    for (const item of landingContent.faq.items) {
      expect(
        screen.getByRole("heading", { level: 3, name: item.question }),
      ).toBeInTheDocument();
    }

    // No stray <h3>s beyond the three data-driven sections above: guards
    // against a section silently dropping or duplicating content entries.
    expect(screen.getAllByRole("heading", { level: 3 })).toHaveLength(
      landingContent.howItWorks.steps.length +
        landingContent.dualMode.columns.length +
        landingContent.faq.items.length,
    );
  });

  it("redirects an authenticated regular user to /dashboard with replace", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user" }),
    });

    renderLandingPage();

    expect(mockNavigate).toHaveBeenCalledWith("/dashboard", { replace: true });
  });

  it("redirects an authenticated admin to /admin with replace", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin" }),
    });

    renderLandingPage();

    expect(mockNavigate).toHaveBeenCalledWith("/admin", { replace: true });
  });
});
