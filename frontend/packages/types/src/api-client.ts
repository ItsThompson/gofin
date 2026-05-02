import type { ApiError } from "./index";

/**
 * Error thrown when an API request fails. Wraps the server's ApiError response
 * and includes the HTTP status code for conditional handling.
 */
export class ApiRequestError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: Record<string, string>;

  constructor(status: number, apiError: ApiError) {
    super(apiError.message);
    this.name = "ApiRequestError";
    this.status = status;
    this.code = apiError.code;
    this.fields = apiError.fields;
  }
}

/** Key used in sessionStorage to save the path the user was on when their session expired. */
const SESSION_RETURN_TO_KEY = "gofin_return_to";

/**
 * Routes that should NOT trigger silent refresh or session expiry redirect.
 * These are auth-related endpoints where a 401 is a domain error (wrong
 * password, invalid credentials) rather than an expired access token.
 * Uses prefix matching so sub-paths (e.g., /api/auth/me/password) are
 * also covered.
 */
const AUTH_ENDPOINT_PREFIXES = [
  "/api/auth/me",
  "/api/auth/login",
  "/api/auth/register",
  "/api/auth/refresh",
];

function isAuthEndpoint(url: string): boolean {
  return AUTH_ENDPOINT_PREFIXES.some(
    (prefix) => url.endsWith(prefix) || url.includes(prefix + "/"),
  );
}

/**
 * In-flight refresh promise. When a 401 triggers a refresh, subsequent
 * 401 retries wait on this same promise instead of firing concurrent
 * refresh requests.
 */
let refreshPromise: Promise<boolean> | null = null;

/**
 * Attempts to refresh the auth tokens by calling POST /api/auth/refresh.
 * Returns true if refresh succeeded, false otherwise.
 */
async function attemptRefresh(): Promise<boolean> {
  try {
    const response = await fetch("/api/auth/refresh", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    });
    return response.ok;
  } catch {
    return false;
  }
}

/**
 * Refreshes auth tokens with concurrency prevention. Only one refresh
 * request fires at a time; concurrent callers share the result.
 */
function refreshTokens(): Promise<boolean> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = attemptRefresh().finally(() => {
    refreshPromise = null;
  });

  return refreshPromise;
}

/**
 * Handles session expiry: saves the current path for post-login redirect
 * and navigates to the login page with an expired indicator.
 */
function handleSessionExpiry(): void {
  if (typeof window === "undefined") return;

  sessionStorage.setItem(SESSION_RETURN_TO_KEY, window.location.pathname);
  window.location.href = "/login?expired=true";
}

/**
 * Core fetch logic shared by the initial request and the retry after refresh.
 */
async function executeFetch<T>(
  url: string,
  options: RequestInit,
): Promise<{ response: Response; data?: T }> {
  const response = await fetch(url, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!response.ok) {
    return { response };
  }

  if (response.status === 204) {
    return { response, data: undefined as T };
  }

  const data = (await response.json()) as T;
  return { response, data };
}

/**
 * Parses an error response into an ApiRequestError.
 */
async function parseErrorResponse(response: Response): Promise<ApiRequestError> {
  let apiError: ApiError;
  try {
    apiError = (await response.json()) as ApiError;
  } catch {
    apiError = {
      code: "UNKNOWN_ERROR",
      message: response.statusText || "An unexpected error occurred",
    };
  }
  return new ApiRequestError(response.status, apiError);
}

/**
 * Shared fetch wrapper that automatically includes credentials (cookies),
 * provides typed error handling, and transparently handles token refresh.
 *
 * On 401 responses (except for auth endpoints), automatically attempts
 * a token refresh and retries the original request. Concurrent 401s
 * share a single refresh call. On refresh failure, redirects to login
 * with session expiry messaging.
 */
export async function apiClient<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const result = await executeFetch<T>(url, options);

  if (result.response.ok) {
    return result.data as T;
  }

  // For non-401 errors, or for auth endpoints, throw immediately
  if (result.response.status !== 401 || isAuthEndpoint(url)) {
    throw await parseErrorResponse(result.response);
  }

  // 401 on a regular API call: attempt silent refresh
  const refreshed = await refreshTokens();

  if (!refreshed) {
    handleSessionExpiry();
    // The redirect navigates away; throw so the caller's await rejects
    // in case the redirect hasn't completed yet (e.g., in tests).
    throw new ApiRequestError(401, {
      code: "SESSION_EXPIRED",
      message: "Your session has expired. Please log in again.",
    });
  }

  // Refresh succeeded: retry the original request
  const retryResult = await executeFetch<T>(url, options);

  if (!retryResult.response.ok) {
    throw await parseErrorResponse(retryResult.response);
  }

  return retryResult.data as T;
}

/**
 * Reads and clears the saved return-to path from sessionStorage.
 * Used by the login page to redirect back after re-authentication.
 */
export function consumeReturnToPath(): string | null {
  if (typeof window === "undefined") return null;
  const path = sessionStorage.getItem(SESSION_RETURN_TO_KEY);
  if (path) {
    sessionStorage.removeItem(SESSION_RETURN_TO_KEY);
  }
  return path;
}
