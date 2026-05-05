import { create } from "zustand";
import type { User } from "@gofin/types";
import { apiClient } from "@gofin/types";

interface AuthResponse {
  user: User;
}

/** Auth state shared across all remotes via Module Federation shared scope. */
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
      });
    } catch (_error) {
      // 401 means not authenticated: expected when no session exists
      set({ ...initialState, isLoading: false });
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
