import { useState, useCallback, useRef } from "react";
import { ApiRequestError } from "../client";
import { classifyApiFailure, NETWORK_FAILURE } from "../errors/classify";
import { reportError } from "../errors/report";
import { isNetworkError, NETWORK_ERROR_MESSAGE } from "./useApiToast";

/** Options for configuring the mutation hook. */
export interface UseFormMutationOptions<T> {
  /** Called with the result after a successful mutation. */
  onSuccess?: (result: T) => void;
  /** Called with the user-facing message and the original error. */
  onError?: (message: string, cause: unknown) => void;
  /** Logical operation for the report, e.g. "expense.create". */
  op?: string;
  /** Business area for the report, e.g. "expenses". */
  domain?: string;
}

/** Return type of useFormMutation. */
export interface FormMutation<T> {
  /** Execute an async operation with automatic state management. */
  submit: (operation: () => Promise<T>) => void;
  /** Whether a submission is currently in progress. */
  submitting: boolean;
  /** Current error message, or null if no error. */
  error: string | null;
  /** Clear the current error. */
  clearError: () => void;
}

/**
 * Hook that consolidates the try/catch/setSubmitting/setError pattern
 * for form mutations. Provides automatic error classification:
 *
 * - ApiRequestError → error.message (server's user-facing message)
 * - Network error → NETWORK_ERROR_MESSAGE
 * - Unknown error → generic fallback message
 *
 * Usage:
 * ```tsx
 * const mutation = useFormMutation<AuthResponse>({
 *   onSuccess: (result) => navigate("/dashboard"),
 * });
 *
 * function handleSubmit(event: FormEvent) {
 *   event.preventDefault();
 *   mutation.submit(() => authApi.login(email, password));
 * }
 * ```
 */
export function useFormMutation<T>(
  options?: UseFormMutationOptions<T>,
): FormMutation<T> {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Store options in a ref so `submit` doesn't re-create on option changes
  const optionsRef = useRef(options);
  optionsRef.current = options;

  const submittingRef = useRef(false);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  const submit = useCallback((operation: () => Promise<T>) => {
    if (submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    setError(null);

    operation()
      .then((result) => {
        submittingRef.current = false;
        setSubmitting(false);
        optionsRef.current?.onSuccess?.(result);
      })
      .catch((err: unknown) => {
        const network = isNetworkError(err);
        let message: string;

        if (err instanceof ApiRequestError) {
          message = err.message;
        } else if (network) {
          message = NETWORK_ERROR_MESSAGE;
        } else {
          message = "An unexpected error occurred. Please try again.";
        }

        const options = optionsRef.current;

        reportError(err, {
          ...(network ? NETWORK_FAILURE : classifyApiFailure(err)),
          op: options?.op,
          domain: options?.domain,
          tags:
            err instanceof ApiRequestError
              ? { http_status: err.status }
              : undefined,
          // Field names only. The values are what the user typed, which here can
          // be a monetary amount or a merchant name.
          data:
            err instanceof ApiRequestError
              ? { fields: Object.keys(err.fields ?? {}) }
              : undefined,
        });

        setError(message);
        submittingRef.current = false;
        setSubmitting(false);
        options?.onError?.(message, err);
      });
  }, []);

  return { submit, submitting, error, clearError };
}
