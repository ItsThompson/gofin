/** Core user model returned by the auth API. */
export interface User {
  id: string;
  username: string;
  email: string;
  role: "user" | "admin";
  currency: string;
  hasCompletedOnboarding: boolean;
  createdAt: string;
}

/** API error response shape. All API errors follow this contract. */
export interface ApiError {
  /** Machine-readable error code (e.g., "VALIDATION_ERROR", "NOT_FOUND"). */
  code: string;
  /** Human-readable error message for display. */
  message: string;
  /** Optional field-level validation errors. */
  fields?: Record<string, string>;
}

/** Wrapper for paginated API responses. */
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}
