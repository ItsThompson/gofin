import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { FaqItem } from "../components/FaqItem";

describe("FaqItem", () => {
  it("renders an <h3> question, a paragraph answer, and a top-border divider", () => {
    const { container } = render(
      <FaqItem
        question="How is GoFin different from my banking app?"
        answer="Your bank lists what you spent; GoFin sorts it by priority."
      />,
    );

    const question = screen.getByRole("heading", { level: 3 });
    expect(question).toHaveTextContent(
      "How is GoFin different from my banking app?",
    );

    const answer = screen.getByText(
      "Your bank lists what you spent; GoFin sorts it by priority.",
    );
    expect(answer.tagName).toBe("P");

    // The divider is a top border on the entry wrapper (§06 border-t dividers).
    expect(container.firstElementChild).toHaveClass("border-t");
  });
});
