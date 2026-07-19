import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { HeroSection } from "../components/HeroSection";
import { landingContent } from "../content";

describe("HeroSection", () => {
  it("renders one <h1>, the subheading, the CTA to /register, microcopy, and the hero visual", () => {
    render(
      <MemoryRouter>
        <HeroSection {...landingContent.hero} />
      </MemoryRouter>,
    );

    const headings = screen.getAllByRole("heading", { level: 1 });
    expect(headings).toHaveLength(1);
    expect(headings[0]).toHaveTextContent(landingContent.hero.heading);

    expect(screen.getByText(landingContent.hero.subheading)).toBeInTheDocument();
    expect(screen.getByText(landingContent.hero.ctaFootnote)).toBeInTheDocument();

    const cta = screen.getByRole("link", {
      name: landingContent.hero.primaryCta.label,
    });
    expect(cta).toHaveAttribute("href", "/register");

    expect(
      screen.getByRole("img", { name: landingContent.hero.visualAlt }),
    ).toBeInTheDocument();
  });
});
