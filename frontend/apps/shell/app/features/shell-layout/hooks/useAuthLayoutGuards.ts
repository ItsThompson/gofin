import { useLocation, useMatches, type UIMatch } from "react-router";
import { canUseFinanceFeatures, getLandingPath, type User } from "@gofin/core";
import { canAccess, type RouteAccess } from "@/lib/route-access";
import type { AuthLayoutGuard } from "../types";

/** The route users complete before reaching their finance surface. */
const ONBOARDING_PATH = "/onboarding";

/** The auth state slice the guard needs from the store. */
interface AuthLayoutState {
  user: User | null;
  isAuthenticated: boolean;
  isAssuming: boolean;
  isLoading: boolean;
}

type AccessHandle = { access?: RouteAccess };

/**
 * deepestAccess reads the access level from the most specific matched route's
 * `handle`. An unclassified route falls back to "authenticated" (fail-safe: it
 * still requires a session but no role).
 */
function deepestAccess(matches: UIMatch[]): RouteAccess {
  const matched = [...matches]
    .reverse()
    .find((match) => (match.handle as AccessHandle | undefined)?.access);
  return (matched?.handle as AccessHandle | undefined)?.access ?? "authenticated";
}

/**
 * useAuthLayoutGuards derives the layout's behavior from auth state and the
 * matched route's `handle.access`.
 *
 * Precedence: loading -> unauthenticated -> access (403) -> onboarding. The
 * access check runs before onboarding, so a direct admin on a personal route
 * (including /onboarding) is forbidden rather than redirected. Onboarding is
 * role-driven: only finance-capable users are ever routed through it, so
 * admins are never sent to /onboarding regardless of hasCompletedOnboarding.
 */
export function useAuthLayoutGuards({
  user,
  isAuthenticated,
  isAssuming,
  isLoading,
}: AuthLayoutState): AuthLayoutGuard {
  const matches = useMatches();
  const access = deepestAccess(matches);
  // Access comes from route metadata (the deepest matched handle.access); the
  // current path comes from useLocation. These are intentionally distinct
  // sources: one is the route's declared classification, the other is the live
  // URL, read only to tell whether we are on the onboarding route.
  const { pathname } = useLocation();

  if (isLoading) {
    return { status: "loading" };
  }
  if (!isAuthenticated || !user) {
    return { status: "redirect", to: "/login" };
  }
  if (!canAccess(user, isAssuming, access)) {
    return { status: "forbidden" };
  }

  if (canUseFinanceFeatures(user)) {
    const onOnboarding = pathname === ONBOARDING_PATH;
    if (!user.hasCompletedOnboarding && !onOnboarding) {
      return { status: "redirect", to: ONBOARDING_PATH };
    }
    if (user.hasCompletedOnboarding && onOnboarding) {
      return { status: "redirect", to: getLandingPath(user) };
    }
    if (!user.hasCompletedOnboarding && onOnboarding) {
      return { status: "onboarding" };
    }
  }

  return { status: "ready" };
}
