import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DualModeSection } from "../components/DualModeSection";
import { landingContent } from "../content";

describe("DualModeSection", () => {
  it("renders the <h2> heading and exactly two feature columns from content", () => {
    const { container } = render(
      <DualModeSection {...landingContent.dualMode} />,
    );

    expect(
      screen.getByRole("heading", {
        level: 2,
        name: landingContent.dualMode.heading,
      }),
    ).toBeInTheDocument();

    // Each FeatureColumn renders one <h3> title, so the h3 count is the column
    // count: guards against a section silently dropping a content entry.
    const columnTitles = screen.getAllByRole("heading", { level: 3 });
    expect(columnTitles).toHaveLength(2);

    for (const column of landingContent.dualMode.columns) {
      expect(screen.getByText(column.title)).toBeInTheDocument();
      expect(screen.getByText(column.body)).toBeInTheDocument();
    }

    // Icons are decorative: their meaning is carried by the adjacent text.
    expect(container.querySelectorAll('svg[aria-hidden="true"]')).toHaveLength(
      2,
    );
  });
});
