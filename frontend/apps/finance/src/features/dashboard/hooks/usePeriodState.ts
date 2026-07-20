import { useState, useEffect, useCallback } from "react";
import { ApiRequestError, useApiToast, useFormMutation } from "@gofin/api";
import type { BudgetPeriod, DefaultSettings, CreatePeriodRequest, CreatePeriodResponse } from "@gofin/core";
import type { PeriodStateResult } from "../types";
import { dashboardApi } from "../api";

type InternalStatus = "loading" | "no-period" | "active" | "error";

export function usePeriodState(): PeriodStateResult {
  const [status, setStatus] = useState<InternalStatus>("loading");
  const [period, setPeriod] = useState<BudgetPeriod | null>(null);
  const [defaults, setDefaults] = useState<DefaultSettings | null>(null);
  const [lastCreateResponse, setLastCreateResponse] = useState<CreatePeriodResponse | null>(null);
  const { call: toastCall } = useApiToast();

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => {
      setLastCreateResponse(response);
      setPeriod(response.period);
      setStatus("active");
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
    setStatus("loading");
    try {
      const response = await dashboardApi.getCurrentPeriod(
        currentYear,
        currentMonth,
      );
      setPeriod(response.period);
      setStatus("active");
    } catch (error) {
      if (
        error instanceof ApiRequestError &&
        error.code === "PERIOD_NOT_FOUND"
      ) {
        try {
          const defaultsResponse = await dashboardApi.getDefaults();
          setDefaults(defaultsResponse.defaults);
        } catch {
          setDefaults(null);
        }
        setStatus("no-period");
        return;
      }
      await toastCall(() => Promise.reject(error));
      setStatus("error");
    }
  }, [currentYear, currentMonth, toastCall]);

  useEffect(() => {
    fetchPeriod();
  }, [fetchPeriod]);

  switch (status) {
    case "loading":
      return { status: "loading", retry: fetchPeriod };

    case "no-period":
      return {
        status: "no-period",
        defaults,
        createPeriod,
        creating: createMutation.submitting,
        createError: createMutation.error,
        clearCreateError: createMutation.clearError,
        lastCreateResponse,
        retry: fetchPeriod,
      };

    case "active":
      return {
        status: "active",
        period: period!,
        retry: fetchPeriod,
      };

    case "error":
      return { status: "error", retry: fetchPeriod };
  }
}
