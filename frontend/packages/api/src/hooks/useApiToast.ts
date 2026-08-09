import { useCallback, useRef } from "react";
import { toast } from "sonner";
import { ApiRequestError } from "../client";
import { classifyApiFailure, NETWORK_FAILURE } from "../errors/classify";
import { reportError } from "../errors/report";

/** Determines if an error is a network/connectivity failure. */
export function isNetworkError(error: unknown): boolean {
  if (error instanceof TypeError) {
    const message = error.message.toLowerCase();
    return (
      message.includes("fetch") ||
      message.includes("network") ||
      message.includes("failed to fetch") ||
      message.includes("load failed")
    );
  }
  return false;
}

/** User-facing message for network failures. */
export const NETWORK_ERROR_MESSAGE =
  "Connection lost. Check your internet and try again.";

interface UseApiToastOptions {
  /**
   * When true, failed operations show a "Retry" action in the toast.
   * Defaults to true for GET-like operations. Pass false for mutations
   * where automatic retry could produce side effects.
   */
  retriable?: boolean;
  /** Logical operation for the report, e.g. "expense.list". */
  op?: string;
  /** Business area for the report, e.g. "expenses". */
  domain?: string;
}

interface ApiToastCallbacks<T> {
  /** Execute an API call with automatic error toasting. */
  call: (
    operation: () => Promise<T>,
    options?: { silent?: boolean },
  ) => Promise<T | undefined>;
  /**
   * Execute an API call, showing a toast only on error. Returns
   * the result or undefined on failure. Useful for data fetching
   * where you don't want a success toast.
   */
  callSilent: (operation: () => Promise<T>) => Promise<T | undefined>;
}

/**
 * Hook that wraps API calls with automatic toast notifications on error.
 *
 * Network errors show "Connection lost" messaging. ApiRequestErrors show
 * the server's user-facing message. Unknown errors show a generic fallback.
 *
 * Retriable operations include a "Retry" button in the toast that re-invokes
 * the same operation.
 *
 * Usage:
 * ```tsx
 * const { call } = useApiToast();
 * const data = await call(() => apiClient<Data>('/api/data'));
 * ```
 */
export function useApiToast<T = unknown>(
  hookOptions: UseApiToastOptions = {},
): ApiToastCallbacks<T> {
  const { retriable = true, op, domain } = hookOptions;
  const lastOperationRef = useRef<(() => Promise<T>) | null>(null);
  /**
   * The operation whose failures are being counted, and how many it has had.
   * Identity is the key, so a memoized operation passed straight in reports its
   * first failure and then stays quiet until a success intervenes. Every call
   * site passes an inline arrow, so each invocation is its own chain today.
   */
  const chainRef = useRef<{
    operation: (() => Promise<T>) | null;
    attempts: number;
  }>({ operation: null, attempts: 0 });

  const call = useCallback(
    async (
      operation: () => Promise<T>,
      callOptions?: { silent?: boolean },
    ): Promise<T | undefined> => {
      // A silent call is a plain pass-through by contract: no toast, no report,
      // and no chain bookkeeping, so it cannot suppress a visible call's report
      // or redirect a visible call's Retry action.
      if (callOptions?.silent) {
        try {
          return await operation();
        } catch {
          return undefined;
        }
      }

      lastOperationRef.current = operation;

      // Captured per invocation. Reading the shared count inside the catch would
      // let a concurrent sibling decide this call's outcome, and one consumer
      // fans four calls out on a single hook instance inside one Promise.all.
      const attempt =
        operation === chainRef.current.operation
          ? chainRef.current.attempts + 1
          : 1;
      chainRef.current = { operation, attempts: attempt };

      try {
        const result = await operation();
        // A success ends only the chain it owns, so a sibling still in flight
        // keeps its own count.
        if (chainRef.current.operation === operation) {
          chainRef.current = { operation: null, attempts: 0 };
        }
        return result;
      } catch (error) {
        const network = isNetworkError(error);
        let message: string;

        if (network) {
          message = NETWORK_ERROR_MESSAGE;
        } else if (error instanceof ApiRequestError) {
          message = error.message;
        } else if (error instanceof Error) {
          message = error.message;
        } else {
          message = "An unexpected error occurred. Please try again.";
        }

        // The Retry action re-invokes this same operation and lands back in this
        // catch, so reporting per attempt would turn one incident into one event
        // per click.
        if (attempt === 1) {
          reportError(error, {
            ...(network ? NETWORK_FAILURE : classifyApiFailure(error)),
            op,
            domain,
            // Always 1 while only the first attempt reports. The taxonomy asks
            // for the count on the event, so a change to reporting at chain end
            // has somewhere to put the real figure.
            data: { attempt },
          });
        }

        const toastOptions: Parameters<typeof toast.error>[1] = {};

        if (retriable) {
          toastOptions.action = {
            label: "Retry",
            onClick: () => {
              if (lastOperationRef.current) {
                call(lastOperationRef.current);
              }
            },
          };
        }

        toast.error(message, toastOptions);

        return undefined;
      }
    },
    [retriable, op, domain],
  );

  const callSilent = useCallback(
    (operation: () => Promise<T>) => call(operation, { silent: true }),
    [call],
  );

  return { call, callSilent };
}
