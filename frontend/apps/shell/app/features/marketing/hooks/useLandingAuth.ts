import { useEffect } from "react";
import { useAuthStore } from "@/stores/auth-store";
import type { LandingAuth } from "../types";

/**
 * Runs the auth check on mount and returns the current auth state for the
 * marketing page. It never navigates: an authenticated visitor stays on `/`
 * and the header renders the logged-in view.
 *
 * checkAuth still runs on mount so the header knows the auth state; the page
 * simply reads the resolved state rather than redirecting on it.
 */
export function useLandingAuth(): LandingAuth {
  const { isLoading, isAuthenticated, user, logout, checkAuth } =
    useAuthStore();

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  return { isLoading, isAuthenticated, user, logout };
}
