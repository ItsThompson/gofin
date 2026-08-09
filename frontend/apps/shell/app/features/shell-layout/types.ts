import type { LucideIcon } from "lucide-react";
import type { User } from "@gofin/core";

/** A single navbar entry (desktop and mobile share the same set). */
export interface NavLinkItem {
  to: string;
  label: string;
  icon: LucideIcon;
}

/**
 * Outcome of the auth-layout guard, derived from auth state and the matched
 * route's `handle.access`. The orchestrator renders each status:
 * - `loading`: initial auth check in flight,
 * - `unavailable`: the auth check could not complete, so the session state is
 *   unknown and a redirect to /login would misreport an outage as a logout,
 * - `redirect`: navigate elsewhere (unauthenticated -> /login, onboarding flow),
 * - `forbidden`: render the 403 page inside the layout chrome,
 * - `onboarding`: render the onboarding outlet without chrome,
 * - `ready`: render the full layout and the routed page.
 */
export type AuthLayoutGuard =
  | { status: "loading" }
  | { status: "unavailable" }
  | { status: "redirect"; to: string }
  | { status: "forbidden" }
  | { status: "onboarding" }
  | { status: "ready" };

export interface BackendUnavailableProps {
  /** Re-run the auth check. Awaited, so the button can show it is working. */
  onRetry: () => Promise<void>;
}

export interface NavbarProps {
  user: User;
  isDirectAdmin: boolean;
  onLogout: () => void;
}

export interface DesktopNavProps {
  navLinks: NavLinkItem[];
}

export interface MobileNavProps {
  navLinks: NavLinkItem[];
  user: User;
  open: boolean;
  onNavigate: () => void;
  onLogout: () => void;
}

export interface ReturnToAdminButtonProps {
  onReturn: () => void;
  disabled: boolean;
}
