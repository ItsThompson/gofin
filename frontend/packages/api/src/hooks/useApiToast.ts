import { useCallback, useRef } from "react";
import { toast } from "sonner";
import { ApiRequestError } from "../client";

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
  const { retriable = true } = hookOptions;
  const lastOperationRef = useRef<(() => Promise<T>) | null>(null);

  const call = useCallback(
    async (
      operation: () => Promise<T>,
      callOptions?: { silent?: boolean },
    ): Promise<T | undefined> => {
      lastOperationRef.current = operation;

      try {
        return await operation();
      } catch (error) {
        if (callOptions?.silent) {
          return undefined;
        }

        let message: string;

        if (isNetworkError(error)) {
          message = NETWORK_ERROR_MESSAGE;
        } else if (error instanceof ApiRequestError) {
          message = error.message;
        } else if (error instanceof Error) {
          message = error.message;
        } else {
          message = "An unexpected error occurred. Please try again.";
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
    [retriable],
  );

  const callSilent = useCallback(
    (operation: () => Promise<T>) => call(operation, { silent: true }),
    [call],
  );

  return { call, callSilent };
}
