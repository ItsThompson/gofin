import { useCallback, useEffect, useRef, useState } from "react";
import { ApiRequestError, useApiToast } from "@gofin/api";
import type { BudgetPeriod } from "@gofin/core";
import { fetchDashboardSection, type DashboardSectionPayload } from "./dashboardDataRequests";
import type {
  BreakdownChart,
  DashboardSectionKey,
  DashboardSectionState,
  SectionState,
} from "../types";

const BASE_SECTIONS: DashboardSectionKey[] = [
  "summary",
  "recentExpenses",
  "upcomingProRata",
  "healthScore",
  "healthScoreTrend",
];
const DESKTOP_SECTIONS: DashboardSectionKey[] = [
  "byTag",
  "cumulative",
  "comparison",
  "trends",
];
const DESKTOP_ONLY_SECTIONS: DashboardSectionKey[] = [...DESKTOP_SECTIONS, "suggestions"];

function getDesktopSections(breakdownChart: BreakdownChart): DashboardSectionKey[] {
  return breakdownChart === "repeated-expenses"
    ? [...DESKTOP_SECTIONS, "suggestions"]
    : [...DESKTOP_SECTIONS];
}

interface GenerationRef {
  current: number;
}

interface PeriodRef {
  current: BudgetPeriod;
}

export interface DashboardSectionController {
  sections: DashboardSectionState;
  desktopVisible: boolean;
  trendMonths: 6 | 12;
  setTrendMonths: (months: 6 | 12, shouldReload: boolean) => void;
  breakdownChart: BreakdownChart;
  selectBreakdown: (chart: BreakdownChart, shouldLoad: boolean) => void;
  abortRequests: () => void;
  resetSections: () => void;
  setSectionLoading: (section: DashboardSectionKey) => void;
  clearRequestedSection: (section: DashboardSectionKey) => void;
  loadSection: (section: DashboardSectionKey, force?: boolean) => Promise<void>;
  startPeriodSections: (forceRefresh?: boolean) => void;
}

function isDesktopViewport(): boolean {
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
    suggestions: { status: "idle", suggestions: [], errorMessage: null },
  };
}

function sectionError(error: unknown): { message: string } {
  if (error instanceof ApiRequestError) return { message: error.message };
  if (error instanceof Error) return { message: error.message };
  return { message: "This section is unavailable right now." };
}

function arrayState<T>(value: readonly T[]): SectionState<readonly T[]> {
  return value.length === 0 ? { status: "empty" } : { status: "success", data: value };
}

function applySectionPayload(
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

export function useDashboardSections(
  generationRef: GenerationRef,
  periodRef: PeriodRef,
  periodActive: boolean,
): DashboardSectionController {
  const [desktopVisible, setDesktopVisible] = useState(isDesktopViewport);
  const [sections, setSections] = useState<DashboardSectionState>(() => createInitialDashboardSectionState(desktopVisible));
  const [trendMonths, setTrendMonthsState] = useState<6 | 12>(6);
  const [breakdownChart, setBreakdownChart] = useState<BreakdownChart>("tag-spending");
  const { call: reportSectionFailure } = useApiToast({
    retriable: false,
    op: "dashboard.section",
    domain: "budgets",
  });
  const desktopVisibleRef = useRef(desktopVisible);
  const previousDesktopVisibleRef = useRef(desktopVisible);
  const previousPeriodActiveRef = useRef(periodActive);
  desktopVisibleRef.current = desktopVisible;
  const trendMonthsRef = useRef<6 | 12>(6);
  const breakdownChartRef = useRef<BreakdownChart>("tag-spending");
  const requestedRef = useRef(new Set<DashboardSectionKey>());
  const controllersRef = useRef(new Map<DashboardSectionKey, AbortController>());

  const abortRequests = useCallback(() => {
    for (const controller of controllersRef.current.values()) controller.abort();
    controllersRef.current.clear();
  }, []);

  useEffect(() => () => abortRequests(), [abortRequests]);

  const resetSections = useCallback(() => {
    requestedRef.current.clear();
    setSections(createInitialDashboardSectionState(false));
  }, []);

  const setSectionLoading = useCallback((section: DashboardSectionKey) => {
    if (section === "suggestions") {
      setSections((current) => ({
        ...current,
        suggestions: { status: "loading", suggestions: [], errorMessage: null },
      }));
      return;
    }
    setSections((current) => ({ ...current, [section]: { status: "loading" } }));
  }, []);

  const clearRequestedSection = useCallback((section: DashboardSectionKey) => {
    requestedRef.current.delete(section);
  }, []);

  const loadSection = useCallback(async (section: DashboardSectionKey, force = false) => {
    if (section === "byTag" || section === "cumulative" || section === "comparison" || section === "trends" || section === "suggestions") {
      if (!desktopVisibleRef.current) return;
    }
    if (requestedRef.current.has(section) && !force) return;
    requestedRef.current.add(section);
    const generation = generationRef.current;
    const controller = new AbortController();
    controllersRef.current.get(section)?.abort();
    controllersRef.current.set(section, controller);
    setSectionLoading(section);

    try {
      const result = await fetchDashboardSection(
        section,
        periodRef.current,
        trendMonthsRef.current,
        controller.signal,
        force,
      );
      if (generation !== generationRef.current || controller.signal.aborted) return;
      setSections((current) => applySectionPayload(current, result));
    } catch (error) {
      if (controller.signal.aborted || generation !== generationRef.current) return;
      if (section === "comparison" && error instanceof ApiRequestError && error.code === "PERIOD_NOT_FOUND") {
        setSections((current) => ({ ...current, comparison: { status: "empty" } }));
        return;
      }
      void reportSectionFailure(() => Promise.reject(error), { op: `dashboard.${section}` });
      if (section === "suggestions") {
        setSections((current) => ({
          ...current,
          suggestions: {
            status: "error",
            suggestions: [],
            errorMessage: sectionError(error).message,
          },
        }));
        return;
      }
      setSections((current) => ({ ...current, [section]: { status: "error", error: sectionError(error) } }));
    } finally {
      if (controllersRef.current.get(section) === controller) controllersRef.current.delete(section);
    }
  }, [generationRef, periodRef, reportSectionFailure, setSectionLoading]);

  const startPeriodSections = useCallback((forceRefresh = false) => {
    requestedRef.current.clear();
    setSections(createInitialDashboardSectionState(desktopVisibleRef.current));
    const eligible = desktopVisibleRef.current
      ? [...BASE_SECTIONS, ...getDesktopSections(breakdownChartRef.current)]
      : BASE_SECTIONS;
    for (const section of eligible) void loadSection(section, forceRefresh);
  }, [loadSection]);

  useEffect(() => {
    setDesktopVisible(isDesktopViewport());
    const mediaQuery = typeof window === "undefined" || !window.matchMedia
      ? null
      : window.matchMedia("(min-width: 768px)");
    if (!mediaQuery) return;
    const onChange = (event: MediaQueryListEvent) => {
      desktopVisibleRef.current = event.matches;
      if (!event.matches) {
        for (const section of DESKTOP_ONLY_SECTIONS) {
          controllersRef.current.get(section)?.abort();
          requestedRef.current.delete(section);
        }
        setSections((current) => ({
          ...current,
          byTag: { status: "idle" }, cumulative: { status: "idle" },
          comparison: { status: "idle" }, trends: { status: "idle" },
          suggestions: { status: "idle", suggestions: [], errorMessage: null },
        }));
      }
      setDesktopVisible(event.matches);
    };
    mediaQuery.addEventListener("change", onChange);
    return () => mediaQuery.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    const becameDesktopVisible = desktopVisible && !previousDesktopVisibleRef.current;
    const becamePeriodActive = periodActive && !previousPeriodActiveRef.current;
    previousDesktopVisibleRef.current = desktopVisible;
    previousPeriodActiveRef.current = periodActive;
    if (!periodActive || (!becameDesktopVisible && !becamePeriodActive)) return;
    for (const section of getDesktopSections(breakdownChartRef.current)) void loadSection(section);
  }, [desktopVisible, loadSection, periodActive]);

  const selectBreakdown = useCallback((nextChart: BreakdownChart, shouldLoad: boolean) => {
    breakdownChartRef.current = nextChart;
    setBreakdownChart(nextChart);
    if (nextChart === "repeated-expenses") {
      if (shouldLoad) {
        void loadSection("suggestions", true);
      } else {
        controllersRef.current.get("suggestions")?.abort();
        setSectionLoading("suggestions");
      }
      return;
    }
    controllersRef.current.get("suggestions")?.abort();
    requestedRef.current.delete("suggestions");
    setSections((current) => ({
      ...current,
      suggestions: { status: "idle", suggestions: [], errorMessage: null },
    }));
  }, [loadSection, setSectionLoading]);

  const setTrendMonths = useCallback((months: 6 | 12, shouldReload: boolean) => {
    if (months === trendMonthsRef.current) return;
    trendMonthsRef.current = months;
    setTrendMonthsState(months);
    if (shouldReload && desktopVisibleRef.current) {
      void loadSection("trends", true);
    } else {
      controllersRef.current.get("trends")?.abort();
      setSectionLoading("trends");
    }
  }, [loadSection, setSectionLoading]);

  return {
    sections,
    desktopVisible,
    trendMonths,
    setTrendMonths,
    breakdownChart,
    selectBreakdown,
    abortRequests,
    resetSections,
    setSectionLoading,
    clearRequestedSection,
    loadSection,
    startPeriodSections,
  };
}
