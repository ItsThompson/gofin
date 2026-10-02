import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiRequestError,
  classifyApiFailure,
  isNetworkError,
  NETWORK_FAILURE,
  reportError,
  useFormMutation,
} from "@gofin/api";
import type { BudgetPeriod, CreatePeriodRequest, CreatePeriodResponse } from "@gofin/core";
import { dashboardApi } from "../api";
import { getPeriodErrorMessage, isSamePeriod } from "../utils/dashboardData";
import type { DashboardPeriodRecovery, DashboardControllerStatus, DashboardPeriodState } from "../types";

export interface DashboardPeriodController {
  period: BudgetPeriod;
  status: DashboardControllerStatus;
  verifying: boolean;
  refreshVersion: number;
  summaryRetryVersion: number;
  refresh: () => void;
  retrySummary: () => void;
  onSummaryPeriodNotFound: () => boolean;
  onSummarySuccess: () => void;
  replacePeriodAfterEdit: (period: BudgetPeriod) => void;
  recovery: DashboardPeriodRecovery | null;
  error: string | null;
}

export function useDashboardPeriod(period: BudgetPeriod, readOnly = false): DashboardPeriodController {
  const [state, setState] = useState<DashboardPeriodState>({ status: "active", period });
  const [refreshVersion, setRefreshVersion] = useState(0);
  const [summaryRetryVersion, setSummaryRetryVersion] = useState(0);
  const generationRef = useRef(0);
  const periodRef = useRef(period);
  const previousPeriodRef = useRef(period);
  const verificationControllerRef = useRef<AbortController | null>(null);
  const defaultsControllerRef = useRef<AbortController | null>(null);
  const recoveryAttemptedRef = useRef(false);

  const abortVerification = useCallback(() => {
    verificationControllerRef.current?.abort();
    verificationControllerRef.current = null;
    defaultsControllerRef.current?.abort();
    defaultsControllerRef.current = null;
  }, []);

  const enterPeriodRecovery = useCallback(async (requestGeneration: number) => {
    if (readOnly) {
      setState((current) => ({ status: "not-found", period: current.period, error: "This historical period is no longer available." }));
      return;
    }
    const controller = new AbortController();
    defaultsControllerRef.current = controller;
    try {
      const { defaults } = await dashboardApi.getDefaults({ signal: controller.signal });
      if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
      setState((current) => ({ status: "no-period", period: current.period, defaults }));
    } catch (error) {
      if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
      reportError(error, {
        ...(isNetworkError(error) ? NETWORK_FAILURE : classifyApiFailure(error)),
        op: "budget.defaults",
        domain: "budgets",
      });
      setState((current) => ({ status: "error", period: current.period, error: "Could not load budget settings. Retry to continue." }));
    } finally {
      if (defaultsControllerRef.current === controller) defaultsControllerRef.current = null;
    }
  }, [readOnly]);

  const verify = useCallback((requestGeneration: number, onVerified: (verifiedPeriod: BudgetPeriod) => void) => {
    abortVerification();
    const controller = new AbortController();
    verificationControllerRef.current = controller;
    const { year, month } = periodRef.current;
    void dashboardApi.getCurrentPeriod(year, month, { forceRefresh: true, signal: controller.signal })
      .then(({ period: verifiedPeriod }) => {
        if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
        onVerified(verifiedPeriod);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || requestGeneration !== generationRef.current) return;
        abortVerification();
        if (error instanceof ApiRequestError && error.code === "PERIOD_NOT_FOUND") {
          void enterPeriodRecovery(requestGeneration);
          return;
        }
        reportError(error, {
          ...(isNetworkError(error) ? NETWORK_FAILURE : classifyApiFailure(error)),
          op: "budget.period",
          domain: "budgets",
        });
        setState((current) => ({ status: "error", period: current.period, error: getPeriodErrorMessage(error) }));
      });
  }, [abortVerification, enterPeriodRecovery]);

  const activatePeriod = useCallback((nextPeriod: BudgetPeriod, loadAll: boolean) => {
    abortVerification();
    generationRef.current += 1;
    periodRef.current = nextPeriod;
    setState({ status: "active", period: nextPeriod });
    if (loadAll) setRefreshVersion((version) => version + 1);
  }, [abortVerification]);

  const beginRefresh = useCallback((preserveRecoveryAttempt: boolean) => {
    if (!preserveRecoveryAttempt) recoveryAttemptedRef.current = false;
    abortVerification();
    const requestGeneration = ++generationRef.current;
    setState((current) => ({ status: "loading", period: current.period }));
    if (!preserveRecoveryAttempt) setRefreshVersion((version) => version + 1);
    verify(requestGeneration, (verifiedPeriod) => {
      if (!isSamePeriod(verifiedPeriod, periodRef.current)) {
        activatePeriod(verifiedPeriod, true);
        return;
      }
      setState({ status: "active", period: periodRef.current });
      if (preserveRecoveryAttempt) setSummaryRetryVersion((version) => version + 1);
    });
  }, [abortVerification, activatePeriod, verify]);

  const refresh = useCallback(() => {
    beginRefresh(false);
  }, [beginRefresh]);

  const retrySummary = useCallback(() => {
    if (state.status !== "active") return;
    abortVerification();
    const requestGeneration = generationRef.current;
    setState({ status: "verifying", period: periodRef.current });
    verify(requestGeneration, (verifiedPeriod) => {
      if (!isSamePeriod(verifiedPeriod, periodRef.current)) {
        activatePeriod(verifiedPeriod, true);
        return;
      }
      setState({ status: "active", period: periodRef.current });
      setSummaryRetryVersion((version) => version + 1);
    });
  }, [abortVerification, activatePeriod, state.status, verify]);

  const onSummaryPeriodNotFound = useCallback(() => {
    if (recoveryAttemptedRef.current || state.status !== "active") return false;
    recoveryAttemptedRef.current = true;
    beginRefresh(true);
    return true;
  }, [beginRefresh, state.status]);

  const onSummarySuccess = useCallback(() => {
    recoveryAttemptedRef.current = false;
  }, []);

  const replacePeriodAfterEdit = useCallback((nextPeriod: BudgetPeriod) => {
    recoveryAttemptedRef.current = false;
    activatePeriod(nextPeriod, true);
  }, [activatePeriod]);

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => activatePeriod(response.period, true),
  });
  const createPeriod = useCallback((body: CreatePeriodRequest) => {
    createMutation.submit(() => dashboardApi.createPeriod(body));
  }, [createMutation.submit]);

  useEffect(() => {
    if (isSamePeriod(period, previousPeriodRef.current)) return;
    previousPeriodRef.current = period;
    periodRef.current = period;
    abortVerification();
    generationRef.current += 1;
    recoveryAttemptedRef.current = false;
    setState({ status: "active", period });
    setRefreshVersion((version) => version + 1);
  }, [abortVerification, period]);

  useEffect(() => () => {
    generationRef.current += 1;
    abortVerification();
  }, [abortVerification]);

  const periodChanged = !isSamePeriod(period, previousPeriodRef.current);
  const visibleStatus: DashboardControllerStatus = periodChanged
    ? "loading"
    : state.status === "verifying"
      ? "active"
      : state.status;

  return {
    period: state.period,
    status: visibleStatus,
    verifying: state.status === "verifying",
    refreshVersion,
    summaryRetryVersion,
    refresh,
    retrySummary,
    onSummaryPeriodNotFound,
    onSummarySuccess,
    replacePeriodAfterEdit,
    recovery: state.status === "no-period"
      ? {
          defaults: state.defaults ?? null,
          createPeriod,
          creating: createMutation.submitting,
          createError: createMutation.error,
          clearCreateError: createMutation.clearError,
        }
      : null,
    error: state.status === "error" || state.status === "not-found" ? state.error : null,
  };
}
