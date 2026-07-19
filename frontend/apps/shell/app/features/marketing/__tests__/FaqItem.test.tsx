import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { FaqItem } from "../components/FaqItem";

describe("FaqItem", () => {
  it("renders an <h3> question, a paragraph answer, and a top-border divider", () => {
    const { container } = render(
      <FaqItem question="Is GoFin free to start?" answer="Yes, it is free." />,
    );

    const question = screen.getByRole("heading", { level: 3 });
    expect(question).toHaveTextContent("Is GoFin free to start?");

    const answer = screen.getByText("Yes, it is free.");
    expect(answer.tagName).toBe("P");

    // The divider is a top border on the entry wrapper (§06 border-t dividers).
    expect(container.firstElementChild).toHaveClass("border-t");
  });
});
