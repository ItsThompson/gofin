import { describe, it, expect } from "vitest";
import { navLinksFor } from "../nav-links";

// navLinksFor is the pure helper the navbar uses to pick its link set. A direct
// admin (operator, not assuming) gets the operator surface; a regular user or
// an assumed session (both role=user) gets the finance surface.
describe("navLinksFor", () => {
  it("returns the operator surface for a direct admin", () => {
    const links = navLinksFor(true);
    expect(links.map((link) => link.to)).toEqual(["/admin", "/settings"]);
    expect(links.map((link) => link.label)).toEqual(["Admin", "Settings"]);
  });

  it("returns the finance surface for a regular user or assumed session", () => {
    const links = navLinksFor(false);
    expect(links.map((link) => link.to)).toEqual([
      "/dashboard",
      "/expenses",
      "/history",
      "/settings",
    ]);
    expect(links.map((link) => link.label)).toEqual([
      "Dashboard",
      "Expenses",
      "History",
      "Settings",
    ]);
  });

  it("gives every link an icon", () => {
    for (const link of [...navLinksFor(true), ...navLinksFor(false)]) {
      expect(link.icon).toBeTruthy();
    }
  });
});
