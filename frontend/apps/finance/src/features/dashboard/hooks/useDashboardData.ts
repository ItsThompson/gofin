import { useCallback, useEffect, useRef, useState } from "react";
import { ApiRequestError, useFormMutation } from "@gofin/api";
import type {
  BudgetPeriod,
  CreatePeriodRequest,
  CreatePeriodResponse,
  DefaultSettings,
} from "@gofin/core";
import { dashboardApi } from "../api";
import { fetchDashboardSection } from "./dashboardDataRequests";
import type {
  DashboardControllerStatus,
  DashboardPeriodRecovery,
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

export interface DashboardData {
  summary: import("@gofin/core").PeriodSummary | null;
  tagSpending: import("@gofin/core").TagSpending[];
  cumulativeData: import("@gofin/core").CumulativeSpendPoint[];
  recentExpenses: import("@gofin/core").Expense[];
  comparison: import("@gofin/core").HistoricalComparison | null;
  upcomingProRata: import("@gofin/core").ProRataSchedule[];
  trendData: import("@gofin/core").TrendPoint[] | null;
  healthScore: import("@gofin/core").HealthScore | import("@gofin/core").HealthScoreConfigureBudget | null;
  healthScoreTrend: import("@gofin/core").HealthScoreTrendPoint[] | null;
}

export const EMPTY_DASHBOARD_DATA: DashboardData = {
  summary: null,
  tagSpending: [],
  cumulativeData: [],
  recentExpenses: [],
  comparison: null,
  upcomingProRata: [],
  trendData: null,
  healthScore: null,
  healthScoreTrend: null,
};

export interface DashboardDataResult {
  data: DashboardData;
  sections: DashboardSectionState;
  period: BudgetPeriod;
  periodStatus: DashboardControllerStatus;
  periodError: string | null;
  periodRecovery: DashboardPeriodRecovery | null;
  desktopVisible: boolean;
  loading: boolean;
  refresh: () => void;
  retry: (section: DashboardSectionKey) => void;
  replacePeriodAfterEdit: (period: BudgetPeriod) => void;
  trendMonths: 6 | 12;
  setTrendMonths: (months: 6 | 12) => void;
}

function isDesktopViewport(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) return true;
  return window.matchMedia("(min-width: 768px)").matches;
}

function initialSectionState(desktopVisible: boolean): DashboardSectionState {
  const initial = <T>(): SectionState<T> => ({ status: "idle" });
  const sections: DashboardSectionState = {
    summary: initial(),
    byTag: initial(),
    cumulative: initial(),
    recentExpenses: initial(),
    comparison: initial(),
    upcomingProRata: initial(),
    trends: initial(),
    healthScore: initial(),
    healthScoreTrend: initial(),
  };
  for (const section of BASE_SECTIONS) sections[section] = { status: "loading" };
  if (desktopVisible) {
    for (const section of DESKTOP_SECTIONS) sections[section] = { status: "loading" };
  }
  return sections;
}

function sectionError(error: unknown): { message: string } {
  if (error instanceof ApiRequestError) return { message: error.message };
  if (error instanceof Error) return { message: error.message };
  return { message: "This section is unavailable right now." };
}

function stateData<T>(state: SectionState<T>): T | null {
  return state.status === "success" ? state.data : null;
}

function arrayState<T>(value: readonly T[]): SectionState<readonly T[]> {
  return value.length === 0 ? { status: "empty" } : { status: "success", data: value };
}

export function useDashboardData(period: BudgetPeriod, readOnly = false): DashboardDataResult {
  const [renderedPeriod, setRenderedPeriod] = useState(period);
  const [periodStatus, setPeriodStatus] = useState<DashboardControllerStatus>("active");
  const [periodError, setPeriodError] = useState<string | null>(null);
  const [periodDefaults, setPeriodDefaults] = useState<DefaultSettings | null>(null);
  const [desktopVisible, setDesktopVisible] = useState(isDesktopViewport);
  const [sections, setSections] = useState<DashboardSectionState>(() => initialSectionState(desktopVisible));
  const [trendMonths, setTrendMonthsState] = useState<6 | 12>(6);
  const generationRef = useRef(0);
  const periodRef = useRef(period);
  const desktopVisibleRef = useRef(desktopVisible);
  desktopVisibleRef.current = desktopVisible;
  const trendMonthsRef = useRef<6 | 12>(6);
  const requestedRef = useRef(new Set<DashboardSectionKey>());
  const controllersRef = useRef(new Map<DashboardSectionKey, AbortController>());

  const abortRequests = useCallback(() => {
    for (const controller of controllersRef.current.values()) controller.abort();
    controllersRef.current.clear();
  }, []);

  const setSectionLoading = useCallback((section: DashboardSectionKey) => {
    setSections((current) => ({ ...current, [section]: { status: "loading" } }));
  }, []);

  const loadSection = useCallback(async (section: DashboardSectionKey, force = false) => {
    if (section === "byTag" || section === "cumulative" || section === "comparison" || section === "trends") {
      if (!desktopVisibleRef.current && !force) return;
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
      setSections((current) => {
        if (result.section === "summary" || result.section === "comparison" || result.section === "healthScore") {
          return { ...current, [result.section]: { status: "success", data: result.data } };
        }
        return { ...current, [result.section]: arrayState(result.data) };
      });
    } catch (error) {
      if (controller.signal.aborted || generation !== generationRef.current) return;
      if (section === "comparison" && error instanceof ApiRequestError && error.code === "PERIOD_NOT_FOUND") {
        setSections((current) => ({ ...current, comparison: { status: "empty" } }));
        return;
      }
      setSections((current) => ({ ...current, [section]: { status: "error", error: sectionError(error) } }));
    } finally {
      if (controllersRef.current.get(section) === controller) controllersRef.current.delete(section);
    }
  }, [setSectionLoading]);

  const startPeriodSections = useCallback((forceRefresh = false) => {
    requestedRef.current.clear();
    setSections(initialSectionState(desktopVisibleRef.current));
    const eligible = desktopVisibleRef.current ? [...BASE_SECTIONS, ...DESKTOP_SECTIONS] : BASE_SECTIONS;
    for (const section of eligible) void loadSection(section, forceRefresh);
  }, [loadSection]);

  const activatePeriod = useCallback((nextPeriod: BudgetPeriod, forceRefresh = false) => {
    abortRequests();
    generationRef.current += 1;
    periodRef.current = nextPeriod;
    setRenderedPeriod(nextPeriod);
    setPeriodStatus("active");
    setPeriodError(null);
    setPeriodDefaults(null);
    startPeriodSections(forceRefresh);
  }, [abortRequests, startPeriodSections]);

  useEffect(() => {
    setDesktopVisible(isDesktopViewport());
    const mediaQuery = typeof window === "undefined" || !window.matchMedia
      ? null
      : window.matchMedia("(min-width: 768px)");
    if (!mediaQuery) return;
    const onChange = (event: MediaQueryListEvent) => setDesktopVisible(event.matches);
    mediaQuery.addEventListener("change", onChange);
    return () => mediaQuery.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    activatePeriod(period);
  }, [activatePeriod, period]);

  useEffect(() => {
    if (desktopVisible && periodStatus === "active") {
      for (const section of DESKTOP_SECTIONS) void loadSection(section);
    }
  }, [desktopVisible, loadSection, periodStatus]);

  const refresh = useCallback(() => {
    abortRequests();
    const requestGeneration = ++generationRef.current;
    requestedRef.current.clear();
    setPeriodStatus("loading");
    setPeriodError(null);
    setSections(initialSectionState(false));
    const currentPeriod = periodRef.current;
    void dashboardApi.getCurrentPeriod(currentPeriod.year, currentPeriod.month, { forceRefresh: true })
      .then((response) => {
        if (requestGeneration !== generationRef.current) return;
        activatePeriod(response.period, true);
      })
      .catch(async (error: unknown) => {
        if (requestGeneration !== generationRef.current) return;
        setSections(initialSectionState(false));
        if (error instanceof ApiRequestError && error.code === "PERIOD_NOT_FOUND") {
          if (readOnly) {
            setPeriodStatus("not-found");
            setPeriodError("This historical period is no longer available.");
            return;
          }
          let defaults: DefaultSettings | null = null;
          try {
            defaults = (await dashboardApi.getDefaults()).defaults;
          } catch {
            defaults = null;
          }
          if (requestGeneration !== generationRef.current) return;
          setPeriodDefaults(defaults);
          setPeriodStatus("no-period");
          setPeriodError(null);
          return;
        }
        setPeriodStatus("error");
        setPeriodError(sectionError(error).message);
      });
  }, [abortRequests, activatePeriod, readOnly]);

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => activatePeriod(response.period, true),
  });
  const { submit: submitCreatePeriod, submitting, error: createError, clearError } = createMutation;
  const createPeriod = useCallback((body: CreatePeriodRequest) => {
    submitCreatePeriod(() => dashboardApi.createPeriod(body));
  }, [submitCreatePeriod]);

  const retry = useCallback((section: DashboardSectionKey) => {
    if (section === "summary") {
      refresh();
      return;
    }
    if (periodStatus === "active") void loadSection(section, true);
  }, [loadSection, periodStatus, refresh]);

  const replacePeriodAfterEdit = useCallback((nextPeriod: BudgetPeriod) => {
    activatePeriod(nextPeriod);
  }, [activatePeriod]);

  const setTrendMonths = useCallback((months: 6 | 12) => {
    trendMonthsRef.current = months;
    setTrendMonthsState(months);
    if (periodStatus === "active" && desktopVisible) void loadSection("trends", true);
  }, [desktopVisible, loadSection, periodStatus]);

  const data: DashboardData = {
    summary: stateData(sections.summary),
    tagSpending: stateData(sections.byTag) ?? [],
    cumulativeData: stateData(sections.cumulative) ?? [],
    recentExpenses: stateData(sections.recentExpenses) ?? [],
    comparison: stateData(sections.comparison),
    upcomingProRata: stateData(sections.upcomingProRata) ?? [],
    trendData: stateData(sections.trends),
    healthScore: stateData(sections.healthScore),
    healthScoreTrend: stateData(sections.healthScoreTrend),
  };
  const loading = periodStatus !== "active" || Object.values(sections).some((section) => section.status === "loading");

  return {
    data,
    sections,
    period: renderedPeriod,
    periodStatus,
    periodError,
    periodRecovery: periodStatus === "no-period"
      ? {
          defaults: periodDefaults,
          createPeriod,
          creating: submitting,
          createError,
          clearCreateError: clearError,
        }
      : null,
    desktopVisible,
    loading,
    refresh,
    retry,
    replacePeriodAfterEdit,
    trendMonths,
    setTrendMonths,
  };
}
