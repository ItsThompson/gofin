import { useState, useEffect, useCallback, useRef } from "react";
import { ApiRequestError, useApiToast, useFormMutation } from "@gofin/api";
import type { BudgetPeriod, DefaultSettings, CreatePeriodRequest, CreatePeriodResponse, DefaultsResponse } from "@gofin/core";
import type { PeriodStateResult } from "../types";
import { dashboardApi } from "../api";

type PeriodState =
  | { status: "loading" }
  | { status: "no-period"; defaults: DefaultSettings | null }
  | { status: "active"; period: BudgetPeriod }
  | { status: "error" };

export function usePeriodState(): PeriodStateResult {
  const [state, setState] = useState<PeriodState>({ status: "loading" });
  const requestGenerationRef = useRef(0);
  const { call: toastCall } = useApiToast();
  // A failed defaults request cannot prove there are no saved settings.
  // Keep the recovery form hidden until a retry confirms the defaults state.
  const { call: defaultsCall } = useApiToast<DefaultsResponse>({
    retriable: false,
    op: "budget.defaults",
    domain: "budgets",
  });

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => {
      setState({ status: "active", period: response.period });
    },
  });

  const createPeriod = useCallback(
    (body: CreatePeriodRequest) => {
      createMutation.submit(() => dashboardApi.createPeriod(body));
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [createMutation.submit],
  );

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const fetchPeriod = useCallback(async (forceRefresh = false) => {
    const requestGeneration = ++requestGenerationRef.current;
    setState({ status: "loading" });
    try {
      const response = await dashboardApi.getCurrentPeriod(
        currentYear,
        currentMonth,
        forceRefresh ? { forceRefresh: true } : undefined,
      );
      if (requestGeneration !== requestGenerationRef.current) return;
      setState({ status: "active", period: response.period });
    } catch (error) {
      if (
        error instanceof ApiRequestError &&
        error.code === "PERIOD_NOT_FOUND"
      ) {
        const defaultsResponse = await defaultsCall(() =>
          dashboardApi.getDefaults(),
        );
        if (requestGeneration !== requestGenerationRef.current) return;
        if (!defaultsResponse) {
          setState({ status: "error" });
          return;
        }
        setState({
          status: "no-period",
          defaults: defaultsResponse.defaults,
        });
        return;
      }
      if (requestGeneration !== requestGenerationRef.current) return;
      await toastCall(() => Promise.reject(error));
      if (requestGeneration !== requestGenerationRef.current) return;
      setState({ status: "error" });
    }
  }, [currentYear, currentMonth, toastCall, defaultsCall]);

  const retry = useCallback(() => {
    void fetchPeriod(true);
  }, [fetchPeriod]);

  useEffect(() => {
    void fetchPeriod();
  }, [fetchPeriod]);

  switch (state.status) {
    case "loading":
      return { status: "loading", retry };

    case "no-period":
      return {
        status: "no-period",
        defaults: state.defaults,
        createPeriod,
        creating: createMutation.submitting,
        createError: createMutation.error,
        clearCreateError: createMutation.clearError,
        retry,
      };

    case "active":
      return {
        status: "active",
        period: state.period,
        retry,
      };

    case "error":
      return { status: "error", retry };
  }
}
