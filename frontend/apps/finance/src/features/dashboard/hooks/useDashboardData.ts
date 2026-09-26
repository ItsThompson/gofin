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
  BreakdownChart,
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
  breakdownChart: BreakdownChart;
  selectBreakdown: (chart: BreakdownChart) => void;
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
    suggestions: { status: "idle", suggestions: [], errorMessage: null },
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

function isSamePeriod(first: BudgetPeriod, second: BudgetPeriod): boolean {
  return first.id === second.id
    && first.userId === second.userId
    && first.year === second.year
    && first.month === second.month
    && first.budgetAmount === second.budgetAmount
    && first.reportingCurrencyCode === second.reportingCurrencyCode
    && first.essentialsPercent === second.essentialsPercent
    && first.desiresPercent === second.desiresPercent
    && first.savingsPercent === second.savingsPercent
    && first.createdAt === second.createdAt
    && first.updatedAt === second.updatedAt;
}

export function useDashboardData(period: BudgetPeriod, readOnly = false): DashboardDataResult {
  const [renderedPeriod, setRenderedPeriod] = useState(period);
  const [periodStatus, setPeriodStatus] = useState<DashboardControllerStatus>("active");
  const [periodError, setPeriodError] = useState<string | null>(null);
  const [periodDefaults, setPeriodDefaults] = useState<DefaultSettings | null>(null);
  const [desktopVisible, setDesktopVisible] = useState(isDesktopViewport);
  const [sections, setSections] = useState<DashboardSectionState>(() => initialSectionState(desktopVisible));
  const [trendMonths, setTrendMonthsState] = useState<6 | 12>(6);
  const [breakdownChart, setBreakdownChart] = useState<BreakdownChart>("tag-spending");
  const generationRef = useRef(0);
  const periodRef = useRef(period);
  const desktopVisibleRef = useRef(desktopVisible);
  desktopVisibleRef.current = desktopVisible;
  const trendMonthsRef = useRef<6 | 12>(6);
  const breakdownChartRef = useRef<BreakdownChart>("tag-spending");
  const requestedRef = useRef(new Set<DashboardSectionKey>());
  const controllersRef = useRef(new Map<DashboardSectionKey, AbortController>());
  const periodVerificationControllerRef = useRef<AbortController | null>(null);

  const abortRequests = useCallback(() => {
    for (const controller of controllersRef.current.values()) controller.abort();
    controllersRef.current.clear();
    periodVerificationControllerRef.current?.abort();
    periodVerificationControllerRef.current = null;
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
        if (result.section === "suggestions") {
          return {
            ...current,
            suggestions: result.data.length === 0
              ? { status: "empty", suggestions: [], errorMessage: null }
              : { status: "success", suggestions: result.data, errorMessage: null },
          };
        }
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
  }, [setSectionLoading]);

  const startPeriodSections = useCallback((forceRefresh = false) => {
    requestedRef.current.clear();
    setSections(initialSectionState(desktopVisibleRef.current));
    const eligible = desktopVisibleRef.current ? [...BASE_SECTIONS, ...DESKTOP_SECTIONS] : BASE_SECTIONS;
    if (breakdownChartRef.current === "repeated-expenses") eligible.push("suggestions");
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

  const gatePeriodFailure = useCallback(async (error: unknown, requestGeneration: number) => {
    if (requestGeneration !== generationRef.current) return;
    abortRequests();
    const failureGeneration = ++generationRef.current;
    requestedRef.current.clear();
    setSections(initialSectionState(false));
    setPeriodStatus("loading");
    setPeriodError(null);
    setPeriodDefaults(null);
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
      if (failureGeneration !== generationRef.current) return;
      setPeriodDefaults(defaults);
      setPeriodStatus("no-period");
      setPeriodError(null);
      return;
    }
    setPeriodStatus("error");
    setPeriodError(sectionError(error).message);
  }, [abortRequests, readOnly]);

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
      .catch((error: unknown) => {
        void gatePeriodFailure(error, requestGeneration);
      });
  }, [abortRequests, activatePeriod, gatePeriodFailure]);

  const retrySummary = useCallback(() => {
    if (periodStatus !== "active") return;
    periodVerificationControllerRef.current?.abort();
    const controller = new AbortController();
    periodVerificationControllerRef.current = controller;
    const requestGeneration = generationRef.current;
    requestedRef.current.delete("summary");
    setSectionLoading("summary");

    void dashboardApi.getCurrentPeriod(periodRef.current.year, periodRef.current.month, {
      forceRefresh: true,
      signal: controller.signal,
    }).then((response) => {
      if (requestGeneration !== generationRef.current || controller.signal.aborted) return;
      if (!isSamePeriod(response.period, periodRef.current)) {
        activatePeriod(response.period, true);
        return;
      }
      void loadSection("summary", true);
    }).catch((error: unknown) => {
      if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
      void gatePeriodFailure(error, requestGeneration);
    }).finally(() => {
      if (periodVerificationControllerRef.current === controller) {
        periodVerificationControllerRef.current = null;
      }
    });
  }, [activatePeriod, gatePeriodFailure, loadSection, periodStatus, setSectionLoading]);

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => activatePeriod(response.period, true),
  });
  const { submit: submitCreatePeriod, submitting, error: createError, clearError } = createMutation;
  const createPeriod = useCallback((body: CreatePeriodRequest) => {
    submitCreatePeriod(() => dashboardApi.createPeriod(body));
  }, [submitCreatePeriod]);

  const selectBreakdown = useCallback((nextChart: BreakdownChart) => {
    breakdownChartRef.current = nextChart;
    setBreakdownChart(nextChart);
    if (nextChart === "repeated-expenses" && periodStatus === "active") {
      void loadSection("suggestions", true);
      return;
    }
    controllersRef.current.get("suggestions")?.abort();
    requestedRef.current.delete("suggestions");
    setSections((current) => ({
      ...current,
      suggestions: { status: "idle", suggestions: [], errorMessage: null },
    }));
  }, [loadSection, periodStatus]);

  const retry = useCallback((section: DashboardSectionKey) => {
    if (section === "summary") {
      retrySummary();
      return;
    }
    if (periodStatus === "active") void loadSection(section, true);
  }, [loadSection, periodStatus, retrySummary]);

  const replacePeriodAfterEdit = useCallback((nextPeriod: BudgetPeriod) => {
    activatePeriod(nextPeriod);
  }, [activatePeriod]);

  const setTrendMonths = useCallback((months: 6 | 12) => {
    trendMonthsRef.current = months;
    setTrendMonthsState(months);
    if (periodStatus === "active" && desktopVisible) void loadSection("trends", true);
  }, [desktopVisible, loadSection, periodStatus]);

  const periodReady = isSamePeriod(period, renderedPeriod);
  const visibleSections = periodReady ? sections : initialSectionState(false);
  const visiblePeriodStatus = periodReady ? periodStatus : "loading";
  const data: DashboardData = {
    summary: stateData(visibleSections.summary),
    tagSpending: stateData(visibleSections.byTag) ?? [],
    cumulativeData: stateData(visibleSections.cumulative) ?? [],
    recentExpenses: stateData(visibleSections.recentExpenses) ?? [],
    comparison: stateData(visibleSections.comparison),
    upcomingProRata: stateData(visibleSections.upcomingProRata) ?? [],
    trendData: stateData(visibleSections.trends),
    healthScore: stateData(visibleSections.healthScore),
    healthScoreTrend: stateData(visibleSections.healthScoreTrend),
  };
  const loading = visiblePeriodStatus !== "active" || Object.values(visibleSections).some((section) => section.status === "loading");

  return {
    data,
    sections: visibleSections,
    period: renderedPeriod,
    periodStatus: visiblePeriodStatus,
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
    breakdownChart,
    selectBreakdown,
  };
}
