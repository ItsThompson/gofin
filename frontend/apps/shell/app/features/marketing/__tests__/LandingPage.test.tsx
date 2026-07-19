import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, useNavigate } from "react-router";
import { buildUser } from "@gofin/test-utils";
import type { User } from "@gofin/core";
import { useAuthStore } from "@/stores/auth-store";
import { LandingPage } from "../LandingPage";
import { landingContent } from "../content";

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

const mockNavigate = vi.fn();
const checkAuth = vi.fn();

interface StoreState {
  isLoading: boolean;
  isAuthenticated: boolean;
  user: User | null;
}

function setStore(state: StoreState) {
  (useAuthStore as unknown as Mock).mockReturnValue({ ...state, checkAuth });
}

function renderLandingPage() {
  return render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  (useNavigate as unknown as Mock).mockReturnValue(mockNavigate);
});

describe("LandingPage", () => {
  it("renders the marketing page and does not redirect an unauthenticated visitor", () => {
    setStore({ isLoading: false, isAuthenticated: false, user: null });

    renderLandingPage();

    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: landingContent.hero.heading }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: landingContent.login.label }),
    ).toHaveAttribute("href", "/login");
    expect(
      screen.getByRole("link", { name: landingContent.hero.primaryCta.label }),
    ).toHaveAttribute("href", "/register");

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("redirects an authenticated regular user to /dashboard with replace", () => {
    setStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user" }),
    });

    renderLandingPage();

    expect(mockNavigate).toHaveBeenCalledWith("/dashboard", { replace: true });
  });

  it("redirects an authenticated admin to /admin with replace", () => {
    setStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin" }),
    });

    renderLandingPage();

    expect(mockNavigate).toHaveBeenCalledWith("/admin", { replace: true });
  });
});
