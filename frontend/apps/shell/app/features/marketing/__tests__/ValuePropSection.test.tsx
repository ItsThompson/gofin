import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ValuePropSection } from "../components/ValuePropSection";
import { landingContent } from "../content";

describe("ValuePropSection", () => {
  it("renders the quote, body, and footnote from the provided content", () => {
    render(<ValuePropSection {...landingContent.valueProp} />);

    expect(screen.getByText(landingContent.valueProp.quote)).toBeInTheDocument();
    expect(screen.getByText(landingContent.valueProp.body)).toBeInTheDocument();
    expect(
      screen.getByText(landingContent.valueProp.footnote),
    ).toBeInTheDocument();
  });

  it("exposes the band as a region named by its quote", () => {
    render(<ValuePropSection {...landingContent.valueProp} />);

    expect(
      screen.getByRole("region", { name: landingContent.valueProp.quote }),
    ).toBeInTheDocument();
  });
});
