import type { User } from "./types";

/**
 * Single source of frontend role truth.
 *
 * These helpers are pure, synchronous, and framework-free: no React, no network
 * calls. Login, the auth layout, and Settings consume them instead of repeating
 * `user.role === "admin"` checks.
 *
 * Note: an assumed session sets the store's `user` to the target regular user
 * (`role=user`), so `canUseFinanceFeatures` is `true` while assuming. Assumption
 * is distinguished by the store's `isAssuming` flag, not by role.
 */

/** A regular user owns and can use personal finance features. */
export function canUseFinanceFeatures(user: User): boolean {
  return user.role === "user";
}

/** An operator (admin) can use admin/operational features. */
export function canUseAdminFeatures(user: User): boolean {
  return user.role === "admin";
}

/** Where a freshly authenticated user should land. */
export function getLandingPath(user: User): string {
  return canUseAdminFeatures(user) ? "/admin" : "/dashboard";
}
