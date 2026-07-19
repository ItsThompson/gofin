import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { landingIcons } from "../icons";
import type { LandingIcon } from "../types";

const ICON_KEYS: LandingIcon[] = [
  "receipt",
  "pieChart",
  "target",
  "wallet",
  "lineChart",
];

describe("landingIcons", () => {
  it("maps every LandingIcon key to a renderable lucide component", () => {
    for (const key of ICON_KEYS) {
      const Icon = landingIcons[key];
      const { container, unmount } = render(<Icon />);
      expect(container.querySelector("svg")).toBeInTheDocument();
      unmount();
    }
  });
});
