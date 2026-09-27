import { useCallback, useEffect, useRef, useState } from "react";
import { ApiRequestError, classifyApiFailure, isNetworkError, NETWORK_FAILURE, reportError, useFormMutation } from "@gofin/api";
import type {
  BudgetPeriod,
  CreatePeriodRequest,
  CreatePeriodResponse,
  DefaultSettings,
} from "@gofin/core";
import { dashboardApi } from "../api";
import { createInitialDashboardSectionState, useDashboardSections } from "./useDashboardSections";
import type {
  BreakdownChart,
  DashboardControllerStatus,
  DashboardPeriodRecovery,
  DashboardSectionKey,
  DashboardSectionState,
  SectionState,
} from "../types";

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

function periodErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) return error.message;
  if (error instanceof Error) return error.message;
  return "This period is unavailable right now.";
}

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


function stateData<T>(state: SectionState<T>): T | null {
  return state.status === "success" ? state.data : null;
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
  const [periodVerificationPending, setPeriodVerificationPending] = useState(false);
  const generationRef = useRef(0);
  const periodRef = useRef(period);
  const previousPeriodPropRef = useRef(period);
  const periodPropChanged = !isSamePeriod(period, previousPeriodPropRef.current);
  const periodVerificationControllerRef = useRef<AbortController | null>(null);
  const refreshControllerRef = useRef<AbortController | null>(null);
  const deferredSectionsRef = useRef(new Set<DashboardSectionKey>());
  const {
    sections,
    desktopVisible,
    trendMonths,
    breakdownChart,
    abortRequests: abortSectionRequests,
    resetSections,
    setSectionLoading,
    clearRequestedSection,
    loadSection,
    startPeriodSections,
    selectBreakdown: setSectionBreakdown,
    setTrendMonths: setSectionTrendMonths,
  } = useDashboardSections(
    generationRef,
    periodRef,
    periodStatus === "active" && !periodPropChanged && !periodVerificationPending,
  );

  const abortRequests = useCallback(() => {
    abortSectionRequests();
    periodVerificationControllerRef.current?.abort();
    periodVerificationControllerRef.current = null;
    refreshControllerRef.current?.abort();
    refreshControllerRef.current = null;
    deferredSectionsRef.current.clear();
    setPeriodVerificationPending(false);
  }, [abortSectionRequests]);

  useEffect(() => () => {
    generationRef.current += 1;
    abortSectionRequests();
    periodVerificationControllerRef.current?.abort();
    refreshControllerRef.current?.abort();
  }, [abortSectionRequests]);

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
    activatePeriod(period);
  }, [activatePeriod, period]);

  useEffect(() => {
    previousPeriodPropRef.current = period;
  }, [period]);

  const gatePeriodFailure = useCallback(async (error: unknown, requestGeneration: number) => {
    if (requestGeneration !== generationRef.current) return;
    abortRequests();
    const failureGeneration = ++generationRef.current;
    setPeriodVerificationPending(false);
    resetSections();
    setPeriodStatus("loading");
    setPeriodError(null);
    setPeriodDefaults(null);
    if (error instanceof ApiRequestError && error.code === "PERIOD_NOT_FOUND") {
      if (readOnly) {
        setPeriodStatus("not-found");
        setPeriodError("This historical period is no longer available.");
        return;
      }
      const controller = new AbortController();
      refreshControllerRef.current = controller;
      try {
        const { defaults } = await dashboardApi.getDefaults({ signal: controller.signal });
        if (controller.signal.aborted || failureGeneration !== generationRef.current) return;
        setPeriodDefaults(defaults);
        setPeriodStatus("no-period");
        setPeriodError(null);
      } catch (defaultsError) {
        if (controller.signal.aborted || failureGeneration !== generationRef.current) return;
        reportError(defaultsError, {
          ...(isNetworkError(defaultsError) ? NETWORK_FAILURE : classifyApiFailure(defaultsError)),
          op: "budget.defaults",
          domain: "budgets",
        });
        setPeriodStatus("error");
        setPeriodError("Could not load budget settings. Retry to continue.");
      } finally {
        if (refreshControllerRef.current === controller) refreshControllerRef.current = null;
      }
      return;
    }
    reportError(error, {
      ...(isNetworkError(error) ? NETWORK_FAILURE : classifyApiFailure(error)),
      op: "budget.period",
      domain: "budgets",
    });
    setPeriodStatus("error");
    setPeriodError(periodErrorMessage(error));
  }, [abortRequests, readOnly, resetSections]);

  const refresh = useCallback(() => {
    abortRequests();
    const requestGeneration = ++generationRef.current;
    const controller = new AbortController();
    refreshControllerRef.current = controller;
    resetSections();
    setPeriodStatus("loading");
    setPeriodError(null);
    const currentPeriod = periodRef.current;
    void dashboardApi.getCurrentPeriod(currentPeriod.year, currentPeriod.month, { forceRefresh: true, signal: controller.signal })
      .then((response) => {
        if (requestGeneration !== generationRef.current || controller.signal.aborted) return;
        activatePeriod(response.period, true);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
        void gatePeriodFailure(error, requestGeneration);
      }).finally(() => {
        if (refreshControllerRef.current === controller) refreshControllerRef.current = null;
      });
  }, [abortRequests, activatePeriod, gatePeriodFailure, resetSections]);

  const retrySummary = useCallback(() => {
    if (periodStatus !== "active") return;
    periodVerificationControllerRef.current?.abort();
    const controller = new AbortController();
    periodVerificationControllerRef.current = controller;
    setPeriodVerificationPending(true);
    const requestGeneration = generationRef.current;
    clearRequestedSection("summary");
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
      periodVerificationControllerRef.current = null;
      setPeriodVerificationPending(false);
      void loadSection("summary", true);
      for (const section of deferredSectionsRef.current) void loadSection(section, true);
      deferredSectionsRef.current.clear();
    }).catch((error: unknown) => {
      if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
      void gatePeriodFailure(error, requestGeneration);
    }).finally(() => {
      if (periodVerificationControllerRef.current === controller) {
        periodVerificationControllerRef.current = null;
      }
    });
  }, [activatePeriod, clearRequestedSection, gatePeriodFailure, loadSection, periodStatus, setSectionLoading]);

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => activatePeriod(response.period, true),
  });
  const { submit: submitCreatePeriod, submitting, error: createError, clearError } = createMutation;
  const createPeriod = useCallback((body: CreatePeriodRequest) => {
    submitCreatePeriod(() => dashboardApi.createPeriod(body));
  }, [submitCreatePeriod]);

  const retry = useCallback((section: DashboardSectionKey) => {
    if (section === "summary") {
      retrySummary();
      return;
    }
    if (periodStatus !== "active" || periodPropChanged) return;
    if (periodVerificationControllerRef.current) {
      deferredSectionsRef.current.add(section);
      setSectionLoading(section);
      return;
    }
    void loadSection(section, true);
  }, [loadSection, periodPropChanged, periodStatus, retrySummary, setSectionLoading]);

  const replacePeriodAfterEdit = useCallback((nextPeriod: BudgetPeriod) => {
    activatePeriod(nextPeriod);
  }, [activatePeriod]);

  const setTrendMonths = useCallback((months: 6 | 12) => {
    const canLoad = periodStatus === "active" && !periodPropChanged && !periodVerificationControllerRef.current;
    setSectionTrendMonths(months, Boolean(canLoad));
    if (periodVerificationControllerRef.current) deferredSectionsRef.current.add("trends");
  }, [periodPropChanged, periodStatus, setSectionTrendMonths]);

  const selectBreakdown = useCallback((chart: BreakdownChart) => {
    const canLoad = periodStatus === "active" && !periodPropChanged && !periodVerificationControllerRef.current;
    setSectionBreakdown(chart, Boolean(canLoad));
    if (!periodVerificationControllerRef.current) return;
    if (chart === "repeated-expenses") deferredSectionsRef.current.add("suggestions");
    else deferredSectionsRef.current.delete("suggestions");
  }, [periodPropChanged, periodStatus, setSectionBreakdown]);

  const visibleSections = periodPropChanged
    ? createInitialDashboardSectionState(false)
    : sections;
  const visiblePeriodStatus = periodPropChanged ? "loading" : periodStatus;
  const trendData = stateData(visibleSections.trends);
  const healthScoreTrend = stateData(visibleSections.healthScoreTrend);
  const data: DashboardData = {
    summary: stateData(visibleSections.summary),
    tagSpending: [...(stateData(visibleSections.byTag) ?? [])],
    cumulativeData: [...(stateData(visibleSections.cumulative) ?? [])],
    recentExpenses: [...(stateData(visibleSections.recentExpenses) ?? [])],
    comparison: stateData(visibleSections.comparison),
    upcomingProRata: [...(stateData(visibleSections.upcomingProRata) ?? [])],
    trendData: trendData ? [...trendData] : null,
    healthScore: stateData(visibleSections.healthScore),
    healthScoreTrend: healthScoreTrend ? [...healthScoreTrend] : null,
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
