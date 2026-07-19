import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";

import { Avatar, AvatarFallback } from "../src/components/avatar";

describe("Avatar", () => {
  it("renders the fallback initial on a themed circle", () => {
    render(
      <Avatar>
        <AvatarFallback>A</AvatarFallback>
      </Avatar>,
    );

    const fallback = screen.getByText("A");
    expect(fallback).toBeInTheDocument();
    // Uses brand tokens, not a hardcoded color.
    expect(fallback).toHaveClass("bg-primary");
    expect(fallback).toHaveClass("text-primary-foreground");
  });
});
