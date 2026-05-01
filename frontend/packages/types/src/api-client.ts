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

/**
 * Shared fetch wrapper that automatically includes credentials (cookies)
 * and provides typed error handling. All API calls across MFE apps should
 * use this instead of raw fetch.
 */
export async function apiClient<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(url, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!response.ok) {
    let apiError: ApiError;
    try {
      apiError = (await response.json()) as ApiError;
    } catch {
      apiError = {
        code: "UNKNOWN_ERROR",
        message: response.statusText || "An unexpected error occurred",
      };
    }
    throw new ApiRequestError(response.status, apiError);
  }

  // Handle 204 No Content responses
  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
