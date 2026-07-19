import { useEffect } from "react";
import { useNavigate } from "react-router";
import { getLandingPath } from "@gofin/core";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Checks auth on mount and redirects an authenticated visitor to their
 * role-aware landing path.
 *
 * Mirrors useLoginForm's check-then-redirect pattern (a mount effect that runs
 * checkAuth, then a redirect effect gated on the resolved store) with one
 * deliberate difference: it navigates with `{ replace: true }` so `/` stays out
 * of history and Back does not return to the landing page and re-trigger the
 * redirect (preserving the prior `<Navigate replace />` semantics).
 *
 * Renders nothing and owns no markup. Unauthenticated visitors keep the
 * marketing page.
 */
export function useLandingRedirect(): void {
  const { isLoading, isAuthenticated, user, checkAuth } = useAuthStore();
  const navigate = useNavigate();

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && isAuthenticated && user) {
      navigate(getLandingPath(user), { replace: true });
    }
  }, [isLoading, isAuthenticated, user, navigate]);
}
