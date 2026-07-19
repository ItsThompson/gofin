import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LandingFooter } from "../components/LandingFooter";
import { landingContent } from "../content";

describe("LandingFooter", () => {
  it("renders a footer landmark with the brand wordmark and the tagline", () => {
    render(
      <LandingFooter
        brand={landingContent.brand}
        tagline={landingContent.tagline}
      />,
    );

    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
    expect(screen.getByText(landingContent.brand)).toBeInTheDocument();
    expect(screen.getByText(landingContent.tagline)).toBeInTheDocument();

    const year = new Date().getFullYear();
    expect(
      screen.getByText(
        `© ${year} ${landingContent.brand}. All rights reserved.`,
      ),
    ).toBeInTheDocument();
  });
});
