import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { LandingHeader } from "../components/LandingHeader";
import { landingContent } from "../content";

describe("LandingHeader", () => {
  it("renders the brand wordmark and a login link pointing to /login", () => {
    render(
      <MemoryRouter>
        <LandingHeader
          brand={landingContent.brand}
          login={landingContent.login}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText(landingContent.brand)).toBeInTheDocument();

    const loginLink = screen.getByRole("link", {
      name: landingContent.login.label,
    });
    expect(loginLink).toHaveAttribute("href", "/login");
  });
});
