import { useEffect, useState } from "react";
import { apiClient, ApiRequestError } from "@gofin/api";
import type { BudgetPeriod, PeriodResponse } from "@gofin/core";

export type PeriodContextState =
  | { status: "loading" }
  | { status: "missing" }
  | { status: "error"; message: string }
  | { status: "active"; period: BudgetPeriod };

const PERIOD_LOAD_ERROR = "Failed to load budget period context.";

/**
 * Loads the current budget period for the given year and month. Returns a
 * discriminated union so callers cannot hold contradictory flags (loading +
 * active, missing + error) at the same time.
 */
export function usePeriodContext(
  year: number,
  month: number,
): PeriodContextState {
  const [state, setState] = useState<PeriodContextState>({ status: "loading" });

  useEffect(() => {
    let isMounted = true;

    async function fetchPeriodContext() {
      try {
        const response = await apiClient<PeriodResponse>(
          `/api/finance/periods/current?year=${year}&month=${month}`,
        );
        if (!isMounted) return;
        setState({ status: "active", period: response.period });
      } catch (error) {
        if (!isMounted) return;
        if (
          error instanceof ApiRequestError &&
          error.code === "PERIOD_NOT_FOUND"
        ) {
          setState({ status: "missing" });
          return;
        }
        setState({ status: "error", message: PERIOD_LOAD_ERROR });
      }
    }

    fetchPeriodContext();

    return () => {
      isMounted = false;
    };
  }, [year, month]);

  return state;
}
