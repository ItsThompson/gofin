import { vi } from "vitest";
import type { User } from "@gofin/core";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Shared auth mocking for the marketing auth-aware tests. Each consuming test
 * file still declares its own `vi.mock("@/stores/auth-store", ...)` (hoisted
 * per file); this module centralizes the spies and the state/reset boilerplate
 * so the header auth-branch (logged-out vs logged-in) and the logout action can
 * be driven and asserted from a single place.
 */

/** The subset of auth-store state the marketing auth logic reads. */
export interface MarketingAuthState {
  isLoading: boolean;
  isAuthenticated: boolean;
  user: User | null;
}

/** Spies for the mocked store actions the landing feature invokes. */
export const checkAuth = vi.fn();
export const logout = vi.fn();

/** Point the mocked useAuthStore at the given state (plus stub actions). */
export function setAuthStore(state: MarketingAuthState): void {
  vi.mocked(useAuthStore).mockReturnValue({
    ...state,
    checkAuth,
    logout,
  } as unknown as ReturnType<typeof useAuthStore>);
}

/** Reset all mocks (spies included). Call in beforeEach. */
export function resetAuthMocks(): void {
  vi.clearAllMocks();
}
