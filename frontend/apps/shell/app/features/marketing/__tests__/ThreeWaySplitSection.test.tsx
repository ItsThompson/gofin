import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ThreeWaySplitSection } from "../components/ThreeWaySplitSection";
import { landingContent } from "../content";
import type { ThreeWaySplitContent } from "../types";

describe("ThreeWaySplitSection", () => {
  it("renders the heading, intro, and one card per bucket with locked copy", () => {
    render(<ThreeWaySplitSection {...landingContent.threeWaySplit} />);

    expect(
      screen.getByRole("heading", {
        level: 2,
        name: landingContent.threeWaySplit.heading,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(landingContent.threeWaySplit.intro),
    ).toBeInTheDocument();

    const titles = screen.getAllByRole("heading", { level: 3 });
    expect(titles).toHaveLength(landingContent.threeWaySplit.buckets.length);

    for (const bucket of landingContent.threeWaySplit.buckets) {
      expect(
        screen.getByRole("heading", { level: 3, name: bucket.title }),
      ).toBeInTheDocument();
      expect(screen.getByText(bucket.body)).toBeInTheDocument();
    }
  });

  it("is data-driven: the card count follows the buckets array", () => {
    const content: ThreeWaySplitContent = {
      heading: "Buckets",
      intro: "Intro copy.",
      buckets: [
        {
          accent: "essentials",
          icon: "house",
          title: "Essentials",
          body: "Essentials body.",
        },
        {
          accent: "savings",
          icon: "piggyBank",
          title: "Savings",
          body: "Savings body.",
        },
      ],
    };

    render(<ThreeWaySplitSection {...content} />);

    expect(screen.getAllByRole("heading", { level: 3 })).toHaveLength(2);
  });
});
