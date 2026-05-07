import { useState, useCallback, useRef } from "react";
import { ApiRequestError } from "../client";
import { isNetworkError, NETWORK_ERROR_MESSAGE } from "./useApiToast";

/** Options for configuring the mutation hook. */
export interface UseFormMutationOptions<T> {
  /** Called with the result after a successful mutation. */
  onSuccess?: (result: T) => void;
  /** Called with the error message after a failed mutation. */
  onError?: (error: string) => void;
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
        let message: string;

        if (err instanceof ApiRequestError) {
          message = err.message;
        } else if (isNetworkError(err)) {
          message = NETWORK_ERROR_MESSAGE;
        } else {
          message = "An unexpected error occurred. Please try again.";
        }

        setError(message);
        submittingRef.current = false;
        setSubmitting(false);
        optionsRef.current?.onError?.(message);
      });
  }, []);

  return { submit, submitting, error, clearError };
}
