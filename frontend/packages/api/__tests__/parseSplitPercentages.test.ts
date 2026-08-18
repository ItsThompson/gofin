import { describe, it, expect } from "vitest";
import { parseSplitPercentages } from "../src/hooks/useBudgetSplitForm/utils";

describe("parseSplitPercentages", () => {
  it("parses numeric field strings", () => {
    expect(
      parseSplitPercentages({ essentials: "50", desires: "30", savings: "20" }),
    ).toEqual({ essentials: 50, desires: 30, savings: 20 });
  });

  it("falls back to 0 for empty fields", () => {
    expect(
      parseSplitPercentages({ essentials: "", desires: "", savings: "" }),
    ).toEqual({ essentials: 0, desires: 0, savings: 0 });
  });

  it("falls back to 0 for non-numeric fields", () => {
    expect(
      parseSplitPercentages({ essentials: "abc", desires: "30", savings: "20" }),
    ).toEqual({ essentials: 0, desires: 30, savings: 20 });
  });

  it("preserves negative values", () => {
    expect(
      parseSplitPercentages({ essentials: "-10", desires: "60", savings: "50" }),
    ).toEqual({ essentials: -10, desires: 60, savings: 50 });
  });

  it("truncates decimals to integers", () => {
    expect(
      parseSplitPercentages({ essentials: "50.5", desires: "29.9", savings: "20" }),
    ).toEqual({ essentials: 50, desires: 29, savings: 20 });
  });
});
