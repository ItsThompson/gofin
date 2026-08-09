import { create } from "zustand";
import type { User } from "@gofin/core";
import { apiClient, ApiRequestError } from "@gofin/api";

interface AuthResponse {
  user: User;
}

/**
 * Why the last auth check failed to confirm a session.
 * - `unauthorized`: the backend answered 401, so there is no session.
 * - `unavailable`: the check never got an answer (5xx, proxy failure, network
 *   drop), so whether a session exists is unknown.
 */
export type AuthCheckError = "unauthorized" | "unavailable";

/** Auth state shared across every feature package in the shell's bundle. */
interface AuthState {
  /** Currently authenticated user, or null if not authenticated. */
  user: User | null;
  /** Whether the user has been authenticated (cookies are valid). */
  isAuthenticated: boolean;
  /** Whether the current user has admin role. */
  isAdmin: boolean;
  /** Whether an admin is currently assuming another user's identity. */
  isAssuming: boolean;
  /** The admin's original user data, preserved during identity assumption. */
  originalAdminUser: User | null;
  /** Whether the initial auth check has completed. */
  isLoading: boolean;
  /** Why the last auth check failed, or null if it succeeded or has not run. */
  authError: AuthCheckError | null;
}

interface AuthActions {
  /** Check auth status by calling /api/auth/me. Called on app mount. */
  checkAuth: () => Promise<void>;
  /** Register a new account. Sets cookies server-side. */
  register: (
    username: string,
    email: string,
    password: string,
  ) => Promise<User>;
  /** Log in with email and password. Sets cookies server-side. */
  login: (email: string, password: string) => Promise<User>;
  /** Log out. Clears cookies and blacklists refresh token. */
  logout: () => Promise<void>;
  /** Admin: assume another user's identity. */
  assumeIdentity: (userId: string) => Promise<void>;
  /** Admin: return to original admin identity. */
  restoreIdentity: () => Promise<void>;
}

type AuthStore = AuthState & AuthActions;

const initialState: AuthState = {
  user: null,
  isAuthenticated: false,
  isAdmin: false,
  isAssuming: false,
  originalAdminUser: null,
  isLoading: true,
  authError: null,
};

export const useAuthStore = create<AuthStore>()((set, get) => ({
  ...initialState,

  checkAuth: async () => {
    try {
      const response = await apiClient<AuthResponse>("/api/auth/me");
      const user = response.user;
      set({
        user,
        isAuthenticated: true,
        isAdmin: user.role === "admin",
        isLoading: false,
        authError: null,
      });
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 401) {
        // No session: expected when nobody is logged in.
        set({ ...initialState, isLoading: false, authError: "unauthorized" });
        return;
      }
      // The check never completed, so the session state is unknown. Keeping the
      // known state means an outage does not present itself as a logout.
      set({ isLoading: false, authError: "unavailable" });
    }
  },

  login: async (email: string, password: string) => {
    const response = await apiClient<AuthResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    const user = response.user;
    set({
      user,
      isAuthenticated: true,
      isAdmin: user.role === "admin",
      isLoading: false,
      authError: null,
    });
    return user;
  },

  register: async (
    username: string,
    email: string,
    password: string,
  ) => {
    const response = await apiClient<AuthResponse>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ username, email, password }),
    });
    const user = response.user;
    set({
      user,
      isAuthenticated: true,
      isAdmin: user.role === "admin",
      isLoading: false,
      authError: null,
    });
    return user;
  },

  logout: async () => {
    try {
      await apiClient("/api/auth/logout", { method: "POST" });
    } catch {
      // Best-effort: clear client state even if the server call fails
    }
    set({ ...initialState, isLoading: false });
  },

  assumeIdentity: async (userId: string) => {
    const currentUser = get().user;
    const response = await apiClient<AuthResponse>("/api/auth/assume", {
      method: "POST",
      body: JSON.stringify({ userId }),
    });
    const assumedUser = response.user;
    set({
      user: assumedUser,
      isAuthenticated: true,
      isAdmin: assumedUser.role === "admin",
      isAssuming: true,
      originalAdminUser: currentUser,
    });
  },

  restoreIdentity: async () => {
    const response = await apiClient<AuthResponse>("/api/auth/restore", {
      method: "POST",
    });
    const adminUser = response.user;
    set({
      user: adminUser,
      isAuthenticated: true,
      isAdmin: adminUser.role === "admin",
      isAssuming: false,
      originalAdminUser: null,
    });
  },
}));
