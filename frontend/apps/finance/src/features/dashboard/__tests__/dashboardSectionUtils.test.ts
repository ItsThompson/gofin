import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "@gofin/api";
import { buildPeriodSummary } from "@gofin/test-utils";
import {
  applySectionPayload,
  clearSuggestions,
  createInitialDashboardSectionState,
  getDesktopSections,
  getSectionErrorMessage,
  getSectionsToLoad,
  isDesktopSection,
  isDesktopViewport,
  resetDesktopSections,
  setSectionLoadingState,
} from "../utils/dashboardSectionUtils";
import type { DashboardSectionState } from "../types";

const summary = buildPeriodSummary({ periodId: "period-1", year: 2026, month: 5 });
const originalMatchMedia = Object.getOwnPropertyDescriptor(window, "matchMedia");

function createSections(): DashboardSectionState {
  return createInitialDashboardSectionState(true);
}

describe("dashboard section utilities", () => {
  afterEach(() => {
    if (originalMatchMedia) Object.defineProperty(window, "matchMedia", originalMatchMedia);
    vi.restoreAllMocks();
  });

  it("selects desktop sections based on the breakdown chart", () => {
    expect(getDesktopSections("tag-spending")).not.toContain("suggestions");
    expect(getDesktopSections("repeated-expenses")).toContain("suggestions");
    expect(isDesktopSection("suggestions")).toBe(true);
    expect(isDesktopSection("summary")).toBe(false);
  });

  it("creates the correct initial state for each viewport", () => {
    expect(createInitialDashboardSectionState(false).byTag.status).toBe("idle");
    expect(createInitialDashboardSectionState(true).byTag.status).toBe("loading");
    expect(createInitialDashboardSectionState(false).suggestions).toEqual({
      status: "idle",
      suggestions: [],
      errorMessage: null,
    });
  });

  it("reads the desktop breakpoint without requiring a browser matchMedia implementation", () => {
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      value: vi.fn().mockReturnValue({ matches: false }),
    });
    expect(isDesktopViewport()).toBe(false);

    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      value: undefined,
    });
    expect(isDesktopViewport()).toBe(true);
  });

  it("selects the sections to load for each viewport and chart", () => {
    expect(getSectionsToLoad(false, "tag-spending")).toEqual([
      "summary",
      "recentExpenses",
      "upcomingProRata",
      "healthScore",
      "healthScoreTrend",
    ]);
    expect(getSectionsToLoad(true, "repeated-expenses")).toContain("suggestions");
  });

  it("sets loading state while clearing stale suggestion data", () => {
    const sections = createSections();
    const withSuggestions = {
      ...sections,
      suggestions: { status: "error" as const, suggestions: [], errorMessage: "Failed" },
    };

    expect(setSectionLoadingState(withSuggestions, "summary").summary).toEqual({ status: "loading" });
    expect(setSectionLoadingState(withSuggestions, "suggestions").suggestions).toEqual({
      status: "loading",
      suggestions: [],
      errorMessage: null,
    });
  });

  it("applies empty and successful payloads to their section state", () => {
    const sections = createSections();
    const emptyExpenses = applySectionPayload(sections, { section: "recentExpenses", data: [] });
    expect(emptyExpenses.recentExpenses).toEqual({ status: "empty" });

    const successfulSummary = applySectionPayload(sections, { section: "summary", data: summary });
    expect(successfulSummary.summary).toEqual({ status: "success", data: summary });

    const successfulSuggestions = applySectionPayload(sections, {
      section: "suggestions",
      data: [],
    });
    expect(successfulSuggestions.suggestions).toEqual({
      status: "empty",
      suggestions: [],
      errorMessage: null,
    });
  });

  it("resets desktop sections when the viewport becomes mobile", () => {
    const sections = createSections();
    const resetSections = resetDesktopSections(sections);

    expect(resetSections.byTag).toEqual({ status: "idle" });
    expect(resetSections.comparison).toEqual({ status: "idle" });
    expect(resetSections.suggestions).toEqual({
      status: "idle",
      suggestions: [],
      errorMessage: null,
    });
    expect(clearSuggestions({
      ...sections,
      suggestions: { status: "error", suggestions: [], errorMessage: "Failed" },
    }).suggestions).toEqual(resetSections.suggestions);
  });

  it("turns known and unknown failures into user-facing messages", () => {
    const apiError = new ApiRequestError(500, { code: "SERVER_ERROR", message: "Server failed" });
    expect(getSectionErrorMessage(apiError)).toBe("Server failed");
    expect(getSectionErrorMessage(new Error("Network failed"))).toBe("Network failed");
    expect(getSectionErrorMessage("unexpected")).toBe("This section is unavailable right now.");
  });
});
