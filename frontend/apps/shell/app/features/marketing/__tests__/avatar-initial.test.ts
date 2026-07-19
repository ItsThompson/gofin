import { describe, it, expect } from "vitest";
import { getAvatarInitial } from "../avatar-initial";

describe("getAvatarInitial", () => {
  it("returns the uppercase first letter of a username", () => {
    expect(getAvatarInitial("ada")).toBe("A");
  });

  it("uppercases an already-capitalized initial unchanged", () => {
    expect(getAvatarInitial("Grace")).toBe("G");
  });

  it("ignores leading whitespace", () => {
    expect(getAvatarInitial("  linus")).toBe("L");
  });

  it("returns an empty string for an empty username", () => {
    expect(getAvatarInitial("")).toBe("");
    expect(getAvatarInitial("   ")).toBe("");
  });

  it("handles a numeric leading character", () => {
    expect(getAvatarInitial("42agent")).toBe("4");
  });
});
