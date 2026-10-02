import { describe, expect, it } from "vitest";
import { ApiRequestError } from "@gofin/api";
import { buildPeriod } from "@gofin/test-utils";
import { getPeriodErrorMessage, isSamePeriod } from "../dashboardData";

describe("dashboard data utilities", () => {
  it("compares all period identity fields", () => {
    const period = buildPeriod({ id: "period-1" });
    expect(isSamePeriod(period, { ...period })).toBe(true);
    expect(isSamePeriod(period, { ...period, updatedAt: "later" })).toBe(false);
  });

  it("formats known and unknown period errors", () => {
    const apiError = new ApiRequestError(404, {
      code: "PERIOD_NOT_FOUND",
      message: "Missing period",
    });
    expect(getPeriodErrorMessage(apiError)).toBe("Missing period");
    expect(getPeriodErrorMessage(new Error("Network failure"))).toBe(
      "Network failure",
    );
    expect(getPeriodErrorMessage({})).toBe(
      "This period is unavailable right now.",
    );
  });
});
