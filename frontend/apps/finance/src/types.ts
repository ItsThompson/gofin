import type { User } from "@gofin/types";

/** Props passed to finance remote pages from the shell. */
export interface FinancePageProps {
  user: User;
}

/**
 * Dashboard loading states.
 * - loading: initial fetch in progress
 * - no-period: API returned PERIOD_NOT_FOUND, show creation prompt
 * - active: period exists, show dashboard
 * - error: unexpected error occurred
 */
export type DashboardState = "loading" | "no-period" | "active" | "error";
