import { ApiRequestError } from "@gofin/api";
import type { DashboardSectionPayload } from "../hooks/dashboardDataRequests";
import type {
  BreakdownChart,
  DashboardSectionKey,
  DashboardSectionState,
  ExpenseSuggestionsState,
  SectionState,
} from "../types";

export const BASE_SECTIONS: DashboardSectionKey[] = [
  "summary",
  "recentExpenses",
  "upcomingProRata",
  "healthScore",
  "healthScoreTrend",
];

export const DESKTOP_SECTIONS: DashboardSectionKey[] = [
  "byTag",
  "cumulative",
  "comparison",
  "trends",
];

export const DESKTOP_ONLY_SECTIONS: DashboardSectionKey[] = [...DESKTOP_SECTIONS, "suggestions"];

export function getDesktopSections(breakdownChart: BreakdownChart): DashboardSectionKey[] {
  return breakdownChart === "repeated-expenses"
    ? [...DESKTOP_SECTIONS, "suggestions"]
    : [...DESKTOP_SECTIONS];
}

export function getSectionsToLoad(
  desktopVisible: boolean,
  breakdownChart: BreakdownChart,
): DashboardSectionKey[] {
  return desktopVisible
    ? [...BASE_SECTIONS, ...getDesktopSections(breakdownChart)]
    : [...BASE_SECTIONS];
}

export function isDesktopSection(section: DashboardSectionKey): boolean {
  return DESKTOP_ONLY_SECTIONS.includes(section);
}

export function isDesktopViewport(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) return true;
  return window.matchMedia("(min-width: 768px)").matches;
}

export function createInitialDashboardSectionState(desktopVisible: boolean): DashboardSectionState {
  return {
    summary: { status: "loading" },
    byTag: desktopVisible ? { status: "loading" } : { status: "idle" },
    cumulative: desktopVisible ? { status: "loading" } : { status: "idle" },
    recentExpenses: { status: "loading" },
    comparison: desktopVisible ? { status: "loading" } : { status: "idle" },
    upcomingProRata: { status: "loading" },
    trends: desktopVisible ? { status: "loading" } : { status: "idle" },
    healthScore: { status: "loading" },
    healthScoreTrend: { status: "loading" },
    suggestions: createIdleSuggestionsState(),
  };
}

export function setSectionLoadingState(
  current: DashboardSectionState,
  section: DashboardSectionKey,
): DashboardSectionState {
  if (section === "suggestions") {
    return { ...current, suggestions: { status: "loading", suggestions: [], errorMessage: null } };
  }
  return { ...current, [section]: { status: "loading" } };
}

export function resetDesktopSections(current: DashboardSectionState): DashboardSectionState {
  return {
    ...current,
    byTag: { status: "idle" },
    cumulative: { status: "idle" },
    comparison: { status: "idle" },
    trends: { status: "idle" },
    suggestions: createIdleSuggestionsState(),
  };
}

export function clearSuggestions(current: DashboardSectionState): DashboardSectionState {
  return { ...current, suggestions: createIdleSuggestionsState() };
}

export function getSectionErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) return error.message;
  if (error instanceof Error) return error.message;
  return "This section is unavailable right now.";
}

export function applySectionPayload(
  current: DashboardSectionState,
  result: DashboardSectionPayload,
): DashboardSectionState {
  switch (result.section) {
    case "suggestions":
      return {
        ...current,
        suggestions: result.data.length === 0
          ? { status: "empty", suggestions: [], errorMessage: null }
          : { status: "success", suggestions: result.data, errorMessage: null },
      };
    case "summary":
      return { ...current, summary: { status: "success", data: result.data } };
    case "comparison":
      return { ...current, comparison: { status: "success", data: result.data } };
    case "healthScore":
      return { ...current, healthScore: { status: "success", data: result.data } };
    case "byTag":
      return { ...current, byTag: arrayState(result.data) };
    case "cumulative":
      return { ...current, cumulative: arrayState(result.data) };
    case "recentExpenses":
      return { ...current, recentExpenses: arrayState(result.data) };
    case "upcomingProRata":
      return { ...current, upcomingProRata: arrayState(result.data) };
    case "trends":
      return { ...current, trends: arrayState(result.data) };
    case "healthScoreTrend":
      return { ...current, healthScoreTrend: arrayState(result.data) };
  }
}

function createIdleSuggestionsState(): ExpenseSuggestionsState {
  return { status: "idle", suggestions: [], errorMessage: null };
}

function arrayState<T>(value: readonly T[]): SectionState<readonly T[]> {
  return value.length === 0 ? { status: "empty" } : { status: "success", data: value };
}
