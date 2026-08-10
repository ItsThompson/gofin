import type { User } from "@gofin/core";

/** Props passed to finance pages from the shell. */
export interface FinancePageProps {
  user: User;
}

/** Props for the SettingsPage, extending FinancePageProps with a user-refresh callback. */
export interface SettingsPageProps extends FinancePageProps {
  /** Called after a successful profile or password change so the shell can refresh the auth store. */
  onUserUpdated?: () => void;
}

/**
 * Dashboard loading states.
 * - loading: initial fetch in progress
 * - no-period: API returned PERIOD_NOT_FOUND, show creation prompt
 * - active: period exists, show dashboard
 * - error: unexpected error occurred
 */
export type DashboardState = "loading" | "no-period" | "active" | "error";
