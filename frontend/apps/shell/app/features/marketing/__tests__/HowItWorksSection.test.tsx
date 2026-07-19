import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { HowItWorksSection } from "../components/HowItWorksSection";
import { landingContent } from "../content";

describe("HowItWorksSection", () => {
  it("renders the <h2> heading and exactly one card per content step", () => {
    render(<HowItWorksSection {...landingContent.howItWorks} />);

    expect(
      screen.getByRole("heading", {
        level: 2,
        name: landingContent.howItWorks.heading,
      }),
    ).toBeInTheDocument();

    // Each StepCard renders its title as an <h3>, so counting level-3 headings
    // guards against a section silently dropping content-model entries.
    const stepTitles = screen.getAllByRole("heading", { level: 3 });
    expect(stepTitles).toHaveLength(landingContent.howItWorks.steps.length);

    for (const step of landingContent.howItWorks.steps) {
      expect(
        screen.getByRole("heading", { level: 3, name: step.title }),
      ).toBeInTheDocument();
    }
  });
});
