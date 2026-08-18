import { useState, useEffect, useCallback } from "react";
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
  const { call: toastCall } = useApiToast();
  // The prompt works without saved defaults, but a failed load used to look
  // identical to having none. useApiToast owns the report and the message;
  // retry is off because the toast's Retry discards its result.
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

  const fetchPeriod = useCallback(async () => {
    // A fresh fetch owns no prior data: resetting to loading atomically drops
    // any stale period or defaults instead of letting them survive a retry.
    setState({ status: "loading" });
    try {
      const response = await dashboardApi.getCurrentPeriod(
        currentYear,
        currentMonth,
      );
      setState({ status: "active", period: response.period });
    } catch (error) {
      if (
        error instanceof ApiRequestError &&
        error.code === "PERIOD_NOT_FOUND"
      ) {
        const defaultsResponse = await defaultsCall(() =>
          dashboardApi.getDefaults(),
        );
        setState({
          status: "no-period",
          defaults: defaultsResponse?.defaults ?? null,
        });
        return;
      }
      await toastCall(() => Promise.reject(error));
      setState({ status: "error" });
    }
  }, [currentYear, currentMonth, toastCall, defaultsCall]);

  useEffect(() => {
    fetchPeriod();
  }, [fetchPeriod]);

  switch (state.status) {
    case "loading":
      return { status: "loading", retry: fetchPeriod };

    case "no-period":
      return {
        status: "no-period",
        defaults: state.defaults,
        createPeriod,
        creating: createMutation.submitting,
        createError: createMutation.error,
        clearCreateError: createMutation.clearError,
        retry: fetchPeriod,
      };

    case "active":
      return {
        status: "active",
        period: state.period,
        retry: fetchPeriod,
      };

    case "error":
      return { status: "error", retry: fetchPeriod };
  }
}
