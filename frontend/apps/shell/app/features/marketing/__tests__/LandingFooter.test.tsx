import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LandingFooter } from "../components/LandingFooter";
import { landingContent } from "../content";

describe("LandingFooter", () => {
  it("renders the brand, tagline, and t-industri.es attribution", () => {
    render(
      <LandingFooter
        brand={landingContent.brand}
        tagline={landingContent.tagline}
      />,
    );

    const footer = screen.getByRole("contentinfo");
    expect(footer).toBeInTheDocument();
    expect(screen.getByText(landingContent.brand)).toBeInTheDocument();
    expect(screen.getByText(landingContent.tagline)).toBeInTheDocument();

    // Attribution reads "A t-industri.es project" with t-industri.es linked.
    expect(footer).toHaveTextContent("A t-industri.es project");
    const attributionLink = screen.getByRole("link", { name: "t-industri.es" });
    expect(attributionLink).toHaveAttribute("href", "https://t-industri.es/");
  });

  it("renders no copyright line", () => {
    render(
      <LandingFooter
        brand={landingContent.brand}
        tagline={landingContent.tagline}
      />,
    );

    expect(screen.queryByText(/all rights reserved/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/©/)).not.toBeInTheDocument();
  });
});
