import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { landingIcons } from "../icons";
import type { LandingIcon } from "../types";

const ICON_KEYS = Object.keys(landingIcons) as LandingIcon[];

describe("landingIcons", () => {
  it("maps every LandingIcon key to a renderable lucide component", () => {
    for (const key of ICON_KEYS) {
      const Icon = landingIcons[key];
      const { container, unmount } = render(<Icon />);
      expect(container.querySelector("svg")).toBeInTheDocument();
      unmount();
    }
  });

  it("covers the icon keys added for the new sections", () => {
    for (const key of [
      "gauge",
      "calendarClock",
      "house",
      "sparkles",
      "piggyBank",
    ] as const) {
      expect(landingIcons[key]).toBeDefined();
    }
  });
});
