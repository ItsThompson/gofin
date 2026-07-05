import { canUseAdminFeatures, canUseFinanceFeatures, type User } from "@gofin/core";

/**
 * Access level attached to a route via its `handle`. It mirrors the backend
 * access vocabulary so the shell guard derives behavior from route metadata
 * instead of a hardcoded route list:
 *
 * - `public`: reachable by anyone (login/register live outside the auth layout).
 * - `authenticated`: any signed-in identity, no role check.
 * - `personal`: a regular user's finance surface (assumed sessions carry
 *   role=user, so they pass).
 * - `admin`: an operator acting directly (not while assuming a user).
 */
export type RouteAccess = "public" | "authenticated" | "personal" | "admin";

/**
 * canAccess is the single predicate the auth-layout guard applies to a route's
 * `handle.access`. It generalizes the old FINANCE_ROUTES check into one rule
 * built on the shared role helpers, so adding a new access level never means
 * editing guard branches:
 *
 * - a direct admin fails `personal` (403 instead of a silent redirect),
 * - a regular user fails `admin`,
 * - an assumed session (role=user) passes `personal` and fails `admin`.
 */
export function canAccess(
  user: User,
  isAssuming: boolean,
  access: RouteAccess,
): boolean {
  switch (access) {
    case "public":
    case "authenticated":
      return true;
    case "personal":
      return canUseFinanceFeatures(user);
    case "admin":
      return canUseAdminFeatures(user) && !isAssuming;
  }
}
