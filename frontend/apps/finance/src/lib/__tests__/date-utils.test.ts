import { describe, it, expect, vi, afterEach } from "vitest";
import { toLocalISODate } from "../date-utils";

describe("toLocalISODate", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("formats an injected date as a local YYYY-MM-DD string", () => {
    expect(toLocalISODate(new Date(2026, 0, 5))).toBe("2026-01-05");
  });

  it("zero-pads single-digit months and days", () => {
    expect(toLocalISODate(new Date(2026, 8, 9))).toBe("2026-09-09");
  });

  it("keeps two-digit months and days unpadded", () => {
    expect(toLocalISODate(new Date(2026, 11, 25))).toBe("2026-12-25");
  });

  it("uses today's local date when called with no argument", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 7, 12, 0, 0));

    expect(toLocalISODate()).toBe("2026-03-07");
  });
});
