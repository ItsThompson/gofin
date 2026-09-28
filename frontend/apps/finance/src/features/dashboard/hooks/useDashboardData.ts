import { useCallback, useEffect, useRef, useState } from "react";
import { ApiRequestError, classifyApiFailure, isNetworkError, NETWORK_FAILURE, reportError, useFormMutation } from "@gofin/api";
import type {
  BudgetPeriod,
  CreatePeriodRequest,
  CreatePeriodResponse,
  DefaultSettings,
} from "@gofin/core";
import { dashboardApi } from "../api";
import {
  buildDashboardData,
  getPeriodErrorMessage,
  isDashboardLoading,
  isSamePeriod,
} from "../utils/dashboardData";
import { useDashboardSections } from "./useDashboardSections";
import { createInitialDashboardSectionState } from "../utils/dashboardSectionUtils";
import type {
  BreakdownChart,
  DashboardControllerStatus,
  DashboardDataResult,
  DashboardSectionKey,
} from "../types";

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
  const summaryRecoveryAttemptedRef = useRef(false);
  const summaryRecoveryHandlerRef = useRef<() => void>(() => undefined);
  const handleSummaryPeriodNotFound = useCallback((requestGeneration: number) => {
    if (requestGeneration !== generationRef.current || summaryRecoveryAttemptedRef.current) return false;
    summaryRecoveryAttemptedRef.current = true;
    summaryRecoveryHandlerRef.current();
    return true;
  }, []);
  const handleSummarySuccess = useCallback(() => {
    summaryRecoveryAttemptedRef.current = false;
  }, []);
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
    handleSummaryPeriodNotFound,
    handleSummarySuccess,
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
    summaryRecoveryAttemptedRef.current = false;
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
    setPeriodError(getPeriodErrorMessage(error));
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

  const retrySummary = useCallback((options: { hideStaleData?: boolean } = {}) => {
    if (!options.hideStaleData && periodStatus !== "active") return;
    if (options.hideStaleData) {
      abortSectionRequests();
      generationRef.current += 1;
      resetSections();
      setPeriodStatus("loading");
      setPeriodError(null);
    }
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
      if (options.hideStaleData) {
        setPeriodStatus("active");
        setPeriodError(null);
        startPeriodSections(true);
        deferredSectionsRef.current.clear();
        return;
      }
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
  }, [abortSectionRequests, activatePeriod, clearRequestedSection, gatePeriodFailure, generationRef, loadSection, periodStatus, resetSections, setSectionLoading, startPeriodSections]);

  summaryRecoveryHandlerRef.current = () => retrySummary({ hideStaleData: true });

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
  const data = buildDashboardData(visibleSections);
  const loading = isDashboardLoading(visiblePeriodStatus, visibleSections);

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
