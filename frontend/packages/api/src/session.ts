/** Key used in sessionStorage to save the path the user was on when their session expired. */
const SESSION_RETURN_TO_KEY = "gofin_return_to";

/**
 * Handles session expiry: saves the current path for post-login redirect
 * and navigates to the login page with an expired indicator.
 *
 * Exported for testing and for use by apiClient on refresh failure.
 */
export function handleSessionExpiry(): void {
  if (typeof window === "undefined") return;

  sessionStorage.setItem(SESSION_RETURN_TO_KEY, window.location.pathname);
  window.location.href = "/login?expired=true";
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
