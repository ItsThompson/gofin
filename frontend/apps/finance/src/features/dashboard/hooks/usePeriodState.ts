import { useState, useEffect, useCallback } from "react";
import { ApiRequestError, useApiToast, useFormMutation } from "@gofin/api";
import type { BudgetPeriod, DefaultSettings, CreatePeriodRequest, CreatePeriodResponse } from "@/types";
import { dashboardApi } from "../api";

type PeriodState = "loading" | "no-period" | "active" | "error";

export interface PeriodStateResult {
  state: PeriodState;
  period: BudgetPeriod | null;
  defaults: DefaultSettings | null;
  retry: () => void;
  handlePeriodCreated: (period: BudgetPeriod) => void;
  createPeriod: (body: CreatePeriodRequest) => void;
  creating: boolean;
  createError: string | null;
  clearCreateError: () => void;
  lastCreateResponse: CreatePeriodResponse | null;
}

export function usePeriodState(): PeriodStateResult {
  const [state, setState] = useState<PeriodState>("loading");
  const [period, setPeriod] = useState<BudgetPeriod | null>(null);
  const [defaults, setDefaults] = useState<DefaultSettings | null>(null);
  const [lastCreateResponse, setLastCreateResponse] = useState<CreatePeriodResponse | null>(null);
  const { call: toastCall } = useApiToast();

  const createMutation = useFormMutation<CreatePeriodResponse>({
    onSuccess: (response) => {
      setLastCreateResponse(response);
      setPeriod(response.period);
      setState("active");
    },
  });

  const createPeriod = useCallback(
    (body: CreatePeriodRequest) => {
      createMutation.submit(() => dashboardApi.createPeriod(body));
    },
    [createMutation],
  );

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const fetchPeriod = useCallback(async () => {
    setState("loading");
    try {
      const response = await dashboardApi.getCurrentPeriod(
        currentYear,
        currentMonth,
      );
      setPeriod(response.period);
      setState("active");
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
        setState("no-period");
        return;
      }
      await toastCall(() => Promise.reject(error));
      setState("error");
    }
  }, [currentYear, currentMonth, toastCall]);

  useEffect(() => {
    fetchPeriod();
  }, [fetchPeriod]);

  const handlePeriodCreated = useCallback((newPeriod: BudgetPeriod) => {
    setPeriod(newPeriod);
    setState("active");
  }, []);

  return {
    state,
    period,
    defaults,
    retry: fetchPeriod,
    handlePeriodCreated,
    createPeriod,
    creating: createMutation.submitting,
    createError: createMutation.error,
    clearCreateError: createMutation.clearError,
    lastCreateResponse,
  };
}
