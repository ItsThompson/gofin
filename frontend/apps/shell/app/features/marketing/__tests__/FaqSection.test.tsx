import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { FaqSection } from "../components/FaqSection";
import { landingContent } from "../content";
import type { FaqContent } from "../types";

describe("FaqSection", () => {
  it("renders the <h2> heading and exactly items.length FaqItems from fixture content", () => {
    render(<FaqSection {...landingContent.faq} />);

    expect(
      screen.getByRole("heading", { level: 2, name: landingContent.faq.heading }),
    ).toBeInTheDocument();

    const questions = screen.getAllByRole("heading", { level: 3 });
    expect(questions).toHaveLength(landingContent.faq.items.length);

    for (const item of landingContent.faq.items) {
      expect(
        screen.getByRole("heading", { level: 3, name: item.question }),
      ).toBeInTheDocument();
      expect(screen.getByText(item.answer)).toBeInTheDocument();
    }
  });

  it("is data-driven: the entry count follows the items array, not a hardcoded list", () => {
    const content: FaqContent = {
      heading: "FAQ",
      items: [
        { question: "First question?", answer: "First answer." },
        { question: "Second question?", answer: "Second answer." },
      ],
    };

    render(<FaqSection {...content} />);

    expect(screen.getAllByRole("heading", { level: 3 })).toHaveLength(2);
  });

  it("renders a static list with no expand/collapse controls", () => {
    render(<FaqSection {...landingContent.faq} />);

    // Static list: no accordion/disclosure controls, so no buttons render.
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
