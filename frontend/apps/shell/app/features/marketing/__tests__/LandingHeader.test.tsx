import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { buildUser } from "@gofin/test-utils";
import { LandingHeader } from "../components/LandingHeader";
import { landingContent } from "../content";
import type { LandingAuth } from "../types";

function renderHeader(auth: LandingAuth) {
  return render(
    <MemoryRouter>
      <LandingHeader
        brand={landingContent.brand}
        login={landingContent.login}
        auth={auth}
      />
    </MemoryRouter>,
  );
}

const LOGGED_OUT: LandingAuth = {
  isLoading: false,
  isAuthenticated: false,
  user: null,
  logout: vi.fn(),
};

describe("LandingHeader", () => {
  it("always renders the brand wordmark linking home", () => {
    renderHeader(LOGGED_OUT);
    expect(screen.getByText(landingContent.brand)).toBeInTheDocument();
  });

  it("renders the login link when logged out", () => {
    renderHeader(LOGGED_OUT);

    const loginLink = screen.getByRole("link", {
      name: landingContent.login.label,
    });
    expect(loginLink).toHaveAttribute("href", "/login");
    expect(
      screen.queryByRole("button", { name: "Open account menu" }),
    ).not.toBeInTheDocument();
  });

  it("renders a neutral slot while auth is loading (no login, no avatar)", () => {
    renderHeader({
      isLoading: true,
      isAuthenticated: false,
      user: null,
      logout: vi.fn(),
    });

    expect(
      screen.queryByRole("link", { name: landingContent.login.label }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Open account menu" }),
    ).not.toBeInTheDocument();
  });

  it("renders the Dashboard link and avatar menu when logged in", () => {
    renderHeader({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user", username: "ada" }),
      logout: vi.fn(),
    });

    expect(
      screen.queryByRole("link", { name: landingContent.login.label }),
    ).not.toBeInTheDocument();

    const dashboardLinks = screen.getAllByRole("link", { name: "Dashboard" });
    expect(dashboardLinks[0]).toHaveAttribute("href", "/dashboard");
    expect(
      screen.getByRole("button", { name: "Open account menu" }),
    ).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
  });

  it("points the Dashboard target at /admin for an admin", () => {
    renderHeader({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin", username: "ops" }),
      logout: vi.fn(),
    });

    const dashboardLinks = screen.getAllByRole("link", { name: "Dashboard" });
    for (const link of dashboardLinks) {
      expect(link).toHaveAttribute("href", "/admin");
    }
  });

  it("invokes the store logout action from the dropdown", async () => {
    const user = userEvent.setup();
    const logout = vi.fn();
    renderHeader({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user", username: "ada" }),
      logout,
    });

    await user.click(screen.getByRole("button", { name: "Open account menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "Log out" }));

    expect(logout).toHaveBeenCalledTimes(1);
  });
});
