import { describe, expect, it } from "vitest";
import { ApiRequestError } from "@gofin/api";
import { buildPeriod, buildPeriodSummary } from "@gofin/test-utils";
import type { DashboardSectionState } from "../../types";
import {
  buildDashboardData,
  getPeriodErrorMessage,
  getSectionData,
  isDashboardLoading,
  isSamePeriod,
} from "../dashboardData";

function buildSections(): DashboardSectionState {
  return {
    summary: { status: "idle" },
    byTag: { status: "success", data: [] },
    cumulative: { status: "success", data: [] },
    recentExpenses: { status: "success", data: [] },
    comparison: { status: "idle" },
    upcomingProRata: { status: "success", data: [] },
    trends: { status: "success", data: [] },
    healthScore: { status: "idle" },
    healthScoreTrend: { status: "success", data: [] },
    suggestions: { status: "idle", suggestions: [], errorMessage: null },
  };
}

describe("dashboard data utilities", () => {
  it("returns only successful section data", () => {
    expect(getSectionData({ status: "success", data: "ready" })).toBe("ready");
    expect(getSectionData({ status: "loading" })).toBeNull();
  });

  it("compares all period identity fields", () => {
    const period = buildPeriod({ id: "period-1" });
    expect(isSamePeriod(period, { ...period })).toBe(true);
    expect(isSamePeriod(period, { ...period, updatedAt: "later" })).toBe(false);
  });

  it("builds dashboard data from section state", () => {
    const sections = buildSections();
    const summary = buildPeriodSummary({ periodId: "period-1" });
    sections.summary = { status: "success", data: summary };

    const data = buildDashboardData(sections);

    expect(data.summary).toEqual(summary);
    expect(data.tagSpending).toEqual([]);
    expect(data.recentExpenses).toEqual([]);
    expect(data.trendData).toEqual([]);
    expect(data.healthScoreTrend).toEqual([]);
  });

  it("reports loading when the period or a section is not ready", () => {
    const sections = buildSections();
    expect(isDashboardLoading("loading", sections)).toBe(true);
    expect(isDashboardLoading("active", sections)).toBe(false);
    sections.healthScore = { status: "loading" };
    expect(isDashboardLoading("active", sections)).toBe(true);
  });

  it("formats known and unknown period errors", () => {
    const apiError = new ApiRequestError(404, { code: "PERIOD_NOT_FOUND", message: "Missing period" });
    expect(getPeriodErrorMessage(apiError)).toBe("Missing period");
    expect(getPeriodErrorMessage(new Error("Network failure"))).toBe("Network failure");
    expect(getPeriodErrorMessage({})).toBe("This period is unavailable right now.");
  });
});
