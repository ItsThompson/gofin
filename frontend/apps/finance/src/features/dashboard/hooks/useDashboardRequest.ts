import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiRequestError,
  classifyApiFailure,
  isNetworkError,
  NETWORK_FAILURE,
  reportError,
} from "@gofin/api";
import type { SectionState } from "../types";

interface DashboardRequestOptions<T> {
  enabled: boolean;
  refreshVersion: number;
  retryVersion?: number;
  requestKey?: string | number;
  emptyWhen?: (data: T) => boolean;
  operation: string;
  onPeriodNotFound?: () => boolean;
  periodNotFoundAsEmpty?: boolean;
  loadingWhenDisabled?: boolean;
  forceRefreshOnVersionChange?: boolean;
}

export interface DashboardRequest<T> {
  state: SectionState<T>;
  retry: () => void;
}

export function useDashboardRequest<T>(
  request: (signal: AbortSignal, forceRefresh: boolean) => Promise<T>,
  options: DashboardRequestOptions<T>,
): DashboardRequest<T> {
  const [state, setState] = useState<SectionState<T>>(
    options.enabled ? { status: "loading" } : { status: "idle" },
  );
  const requestIdRef = useRef(0);
  const controllerRef = useRef<AbortController | null>(null);
  const pendingRetryRef = useRef(false);
  const lastRefreshVersionRef = useRef<number | null>(null);
  const lastRetryVersionRef = useRef<number | undefined>(undefined);
  const lastRequestKeyRef = useRef<string | number | undefined>(undefined);
  const wasDisabledRef = useRef(!options.enabled);
  const requestRef = useRef(request);
  requestRef.current = request;

  const load = useCallback(
    (forceRefresh: boolean) => {
      if (!options.enabled) {
        pendingRetryRef.current = true;
        setState({ status: "loading" });
        return;
      }
      pendingRetryRef.current = false;
      controllerRef.current?.abort();
      const controller = new AbortController();
      controllerRef.current = controller;
      const requestId = ++requestIdRef.current;
      setState({ status: "loading" });
      void requestRef
        .current(controller.signal, forceRefresh)
        .then((data) => {
          if (controller.signal.aborted || requestId !== requestIdRef.current)
            return;
          setState(
            options.emptyWhen?.(data)
              ? { status: "empty" }
              : { status: "success", data },
          );
        })
        .catch((error: unknown) => {
          if (controller.signal.aborted || requestId !== requestIdRef.current)
            return;
          if (
            error instanceof ApiRequestError &&
            error.code === "PERIOD_NOT_FOUND"
          ) {
            if (options.periodNotFoundAsEmpty) {
              setState({ status: "empty" });
              return;
            }
            if (options.onPeriodNotFound?.() === true) return;
          }
          reportError(error, {
            ...(isNetworkError(error)
              ? NETWORK_FAILURE
              : classifyApiFailure(error)),
            op: options.operation,
            domain: "budgets",
            data: { attempt: 1 },
          });
          setState({
            status: "error",
            error: {
              message:
                error instanceof Error
                  ? error.message
                  : "This section is unavailable right now.",
            },
          });
        })
        .finally(() => {
          if (controllerRef.current === controller)
            controllerRef.current = null;
        });
    },
    [options],
  );

  useEffect(() => {
    if (!options.enabled) {
      wasDisabledRef.current = true;
      controllerRef.current?.abort();
      if (options.loadingWhenDisabled && state.status !== "loading")
        setState({ status: "loading" });
      return;
    }
    const resumed = wasDisabledRef.current;
    wasDisabledRef.current = false;
    const hasLoadedBefore = lastRefreshVersionRef.current !== null;
    const initial = !hasLoadedBefore;
    const versionChanged =
      hasLoadedBefore &&
      (lastRefreshVersionRef.current !== options.refreshVersion ||
        lastRetryVersionRef.current !== options.retryVersion ||
        lastRequestKeyRef.current !== options.requestKey);
    lastRefreshVersionRef.current = options.refreshVersion;
    lastRetryVersionRef.current = options.retryVersion;
    lastRequestKeyRef.current = options.requestKey;
    const unfinishedResumedRequest = resumed && state.status === "loading";
    if (
      pendingRetryRef.current ||
      state.status === "idle" ||
      initial ||
      versionChanged ||
      unfinishedResumedRequest
    ) {
      const pendingRetry = pendingRetryRef.current;
      const forceRefresh =
        pendingRetry ||
        (initial && options.refreshVersion > 0) ||
        (versionChanged && options.forceRefreshOnVersionChange === true);
      load(forceRefresh);
    }
  }, [
    load,
    options.enabled,
    options.refreshVersion,
    options.retryVersion,
    options.requestKey,
    state.status,
  ]);

  useEffect(
    () => () => {
      controllerRef.current?.abort();
      requestIdRef.current += 1;
    },
    [],
  );

  const requestVersionChanged =
    lastRefreshVersionRef.current !== null &&
    (lastRefreshVersionRef.current !== options.refreshVersion ||
      lastRetryVersionRef.current !== options.retryVersion ||
      lastRequestKeyRef.current !== options.requestKey);
  const visibleState: SectionState<T> = requestVersionChanged
    ? { status: "loading" }
    : state;

  return { state: visibleState, retry: () => load(true) };
}
