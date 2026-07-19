import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StepCard } from "../components/StepCard";
import { landingContent } from "../content";

describe("StepCard", () => {
  const step = landingContent.howItWorks.steps[0];

  it("renders the ordinal, an <h3> title, and a body paragraph", () => {
    render(<StepCard {...step} />);

    expect(screen.getByText(step.ordinal)).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 3, name: step.title }),
    ).toBeInTheDocument();
    expect(screen.getByText(step.body)).toBeInTheDocument();
  });

  it("resolves the icon from the content key and marks it decorative (aria-hidden)", () => {
    const { container } = render(<StepCard {...step} />);

    const icon = container.querySelector("svg");
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });
});
