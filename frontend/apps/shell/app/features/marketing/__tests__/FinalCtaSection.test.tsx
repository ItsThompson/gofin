import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { FinalCtaSection } from "../components/FinalCtaSection";
import { landingContent } from "../content";

describe("FinalCtaSection", () => {
  it("renders one <h2> from heading and a primary CTA linking to /register", () => {
    render(
      <MemoryRouter>
        <FinalCtaSection {...landingContent.finalCta} />
      </MemoryRouter>,
    );

    const headings = screen.getAllByRole("heading", { level: 2 });
    expect(headings).toHaveLength(1);
    expect(headings[0]).toHaveTextContent(landingContent.finalCta.heading);

    const cta = screen.getByRole("link", {
      name: landingContent.finalCta.primaryCta.label,
    });
    expect(cta).toHaveAttribute("href", "/register");
    // asChild renders the Button as a real anchor, not a <button>.
    expect(cta.tagName).toBe("A");
  });

  it("uses the same Button variant and size as the hero CTA (default, lg)", () => {
    render(
      <MemoryRouter>
        <FinalCtaSection {...landingContent.finalCta} />
      </MemoryRouter>,
    );

    const cta = screen.getByRole("link", {
      name: landingContent.finalCta.primaryCta.label,
    });
    expect(cta).toHaveAttribute("data-variant", "default");
    expect(cta).toHaveAttribute("data-size", "lg");
  });
});
