import { useCallback, useEffect, useRef, useState } from "react";
import { ApiRequestError, classifyApiFailure, isNetworkError, NETWORK_FAILURE, reportError, useFormMutation } from "@gofin/api";
import type {
  BudgetPeriod,
  CreatePeriodRequest,
  CreatePeriodResponse,
} from "@gofin/core";
import { dashboardApi } from "../api";
import { getPeriodErrorMessage, isSamePeriod } from "../utils/dashboardData";
import { useDashboardSections } from "./useDashboardSections";
import { createInitialDashboardSectionState } from "../utils/dashboardSectionUtils";
import type {
  BreakdownChart,
  DashboardControllerStatus,
  DashboardDataResult,
  DashboardPeriodState,
  DashboardSectionKey,
} from "../types";

export function useDashboardData(period: BudgetPeriod, readOnly = false): DashboardDataResult {
  const [periodState, setPeriodState] = useState<DashboardPeriodState>({ status: "active", period });
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
    periodState.status === "active" && !periodPropChanged,
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
    setPeriodState({ status: "active", period: nextPeriod });
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
    resetSections();
    setPeriodState({ status: "loading", period: periodRef.current });
    if (error instanceof ApiRequestError && error.code === "PERIOD_NOT_FOUND") {
      if (readOnly) {
        setPeriodState({
          status: "not-found",
          period: periodRef.current,
          error: "This historical period is no longer available.",
        });
        return;
      }
      const controller = new AbortController();
      refreshControllerRef.current = controller;
      try {
        const { defaults } = await dashboardApi.getDefaults({ signal: controller.signal });
        if (controller.signal.aborted || failureGeneration !== generationRef.current) return;
        setPeriodState({ status: "no-period", period: periodRef.current, defaults });
      } catch (defaultsError) {
        if (controller.signal.aborted || failureGeneration !== generationRef.current) return;
        reportError(defaultsError, {
          ...(isNetworkError(defaultsError) ? NETWORK_FAILURE : classifyApiFailure(defaultsError)),
          op: "budget.defaults",
          domain: "budgets",
        });
        setPeriodState({
          status: "error",
          period: periodRef.current,
          error: "Could not load budget settings. Retry to continue.",
        });
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
    setPeriodState({ status: "error", period: periodRef.current, error: getPeriodErrorMessage(error) });
  }, [abortRequests, readOnly, resetSections]);

  const refresh = useCallback(() => {
    abortRequests();
    const requestGeneration = ++generationRef.current;
    const controller = new AbortController();
    refreshControllerRef.current = controller;
    resetSections();
    setPeriodState({ status: "loading", period: periodRef.current });
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
    if (!options.hideStaleData && periodState.status !== "active") return;
    if (options.hideStaleData) {
      abortSectionRequests();
      generationRef.current += 1;
      resetSections();
      setPeriodState({ status: "loading", period: periodRef.current });
    }
    periodVerificationControllerRef.current?.abort();
    const controller = new AbortController();
    periodVerificationControllerRef.current = controller;
    if (!options.hideStaleData) {
      setPeriodState({ status: "verifying", period: periodRef.current });
    }
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
      setPeriodState({ status: "active", period: periodRef.current });
      if (options.hideStaleData) {
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
  }, [abortSectionRequests, activatePeriod, clearRequestedSection, gatePeriodFailure, generationRef, loadSection, periodState.status, resetSections, setSectionLoading, startPeriodSections]);

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
    if (periodPropChanged || (periodState.status !== "active" && periodState.status !== "verifying")) return;
    if (periodVerificationControllerRef.current) {
      deferredSectionsRef.current.add(section);
      setSectionLoading(section);
      return;
    }
    void loadSection(section, true);
  }, [loadSection, periodPropChanged, periodState.status, retrySummary, setSectionLoading]);

  const replacePeriodAfterEdit = useCallback((nextPeriod: BudgetPeriod) => {
    activatePeriod(nextPeriod);
  }, [activatePeriod]);

  const setTrendMonths = useCallback((months: 6 | 12) => {
    const canLoad = periodState.status === "active" && !periodPropChanged && !periodVerificationControllerRef.current;
    setSectionTrendMonths(months, Boolean(canLoad));
    if (periodVerificationControllerRef.current) deferredSectionsRef.current.add("trends");
  }, [periodPropChanged, periodState.status, setSectionTrendMonths]);

  const selectBreakdown = useCallback((chart: BreakdownChart) => {
    const canLoad = periodState.status === "active" && !periodPropChanged && !periodVerificationControllerRef.current;
    setSectionBreakdown(chart, Boolean(canLoad));
    if (!periodVerificationControllerRef.current) return;
    if (chart === "repeated-expenses") deferredSectionsRef.current.add("suggestions");
    else deferredSectionsRef.current.delete("suggestions");
  }, [periodPropChanged, periodState.status, setSectionBreakdown]);

  const visibleSections = periodPropChanged
    ? createInitialDashboardSectionState(false)
    : sections;
  let visiblePeriodStatus: DashboardControllerStatus = periodState.status === "verifying"
    ? "active"
    : periodState.status;
  if (periodPropChanged) visiblePeriodStatus = "loading";

  return {
    sections: visibleSections,
    period: periodState.period,
    periodStatus: visiblePeriodStatus,
    periodError: periodState.status === "error" || periodState.status === "not-found" ? periodState.error : null,
    periodRecovery: periodState.status === "no-period"
      ? {
          defaults: periodState.defaults,
          createPeriod,
          creating: submitting,
          createError,
          clearCreateError: clearError,
        }
      : null,
    desktopVisible,
    refresh,
    retry,
    replacePeriodAfterEdit,
    trendMonths,
    setTrendMonths,
    breakdownChart,
    selectBreakdown,
  };
}
