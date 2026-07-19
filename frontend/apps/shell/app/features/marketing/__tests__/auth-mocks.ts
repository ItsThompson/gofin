import { vi } from "vitest";
import { useNavigate } from "react-router";
import type { User } from "@gofin/core";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Shared auth mocking for the marketing redirect tests. Each consuming test
 * file still declares its own `vi.mock("react-router", ...)` and
 * `vi.mock("@/stores/auth-store", ...)` (those calls are hoisted per file);
 * this module centralizes the spies and the state/reset boilerplate so the
 * `{ replace: true }` redirect can be asserted from a single place.
 */

/** The subset of auth-store state the marketing redirect logic reads. */
export interface MarketingAuthState {
  isLoading: boolean;
  isAuthenticated: boolean;
  user: User | null;
}

/** Spies for the mocked react-router navigate and store.checkAuth. */
export const mockNavigate = vi.fn();
export const checkAuth = vi.fn();

/** Point the mocked useAuthStore at the given state (plus a stub checkAuth). */
export function setAuthStore(state: MarketingAuthState): void {
  vi.mocked(useAuthStore).mockReturnValue({
    ...state,
    checkAuth,
  } as unknown as ReturnType<typeof useAuthStore>);
}

/** Reset all mocks and re-wire useNavigate to the shared spy. Call in beforeEach. */
export function resetAuthMocks(): void {
  vi.clearAllMocks();
  vi.mocked(useNavigate).mockReturnValue(mockNavigate);
}
