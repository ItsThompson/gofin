import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { FeatureShowcaseSection } from "../components/FeatureShowcaseSection";
import { landingContent } from "../content";
import type { FeatureShowcaseContent } from "../types";

describe("FeatureShowcaseSection", () => {
  it("renders the heading and one entry per feature with locked copy", () => {
    render(<FeatureShowcaseSection {...landingContent.featureShowcase} />);

    expect(
      screen.getByRole("heading", {
        level: 2,
        name: landingContent.featureShowcase.heading,
      }),
    ).toBeInTheDocument();

    const titles = screen.getAllByRole("heading", { level: 3 });
    expect(titles).toHaveLength(landingContent.featureShowcase.features.length);

    for (const feature of landingContent.featureShowcase.features) {
      expect(
        screen.getByRole("heading", { level: 3, name: feature.title }),
      ).toBeInTheDocument();
      expect(screen.getByText(feature.body)).toBeInTheDocument();
    }
  });

  it("is data-driven: the entry count follows the features array", () => {
    const content: FeatureShowcaseContent = {
      heading: "Features",
      features: [
        { icon: "gauge", title: "One", body: "First body." },
        { icon: "lineChart", title: "Two", body: "Second body." },
        { icon: "calendarClock", title: "Three", body: "Third body." },
        { icon: "target", title: "Four", body: "Fourth body." },
      ],
    };

    render(<FeatureShowcaseSection {...content} />);

    expect(screen.getAllByRole("heading", { level: 3 })).toHaveLength(4);
  });
});
