import { useState, useEffect, useCallback } from "react";
import { apiClient, useApiToast } from "@gofin/api";
import { useUserDeletion } from "./useUserDeletion";
import type { UserDeletionState, UserDeletionActions } from "./useUserDeletion";
import type { User, AdminUserSummary, AdminUsersResponse } from "@gofin/core";

type AdminLoadState = "loading" | "error" | "success";

export interface AdminPanelState {
  loadState: AdminLoadState;
  users: AdminUserSummary[];
  /** User ID currently being assumed (for loading indicator). */
  assumingUserId: string | null;
  deletion: UserDeletionState;
}

export interface AdminPanelActions {
  handleAssume: (userId: string) => void;
  /** Retry fetching users after error. */
  retry: () => void;
  deletion: UserDeletionActions;
}

export interface UseAdminPanelOptions {
  currentUser: User | null;
  onAssumeIdentity: (userId: string) => Promise<void>;
}

export function useAdminPanel(options: UseAdminPanelOptions): {
  state: AdminPanelState;
  actions: AdminPanelActions;
} {
  const { onAssumeIdentity } = options;
  const [users, setUsers] = useState<AdminUserSummary[]>([]);
  const [loadState, setLoadState] = useState<AdminLoadState>("loading");
  const [assumingUserId, setAssumingUserId] = useState<string | null>(null);
  const { call: toastCall } = useApiToast<AdminUsersResponse>();

  const handleUserRemoved = useCallback((userId: string) => {
    setUsers((prev) => prev.filter((user) => user.id !== userId));
  }, []);

  const { state: deletionState, actions: deletionActions } = useUserDeletion({
    onUserRemoved: handleUserRemoved,
  });

  const fetchUsers = useCallback(async () => {
    setLoadState("loading");
    const result = await toastCall(async () => {
      const response = await apiClient<AdminUsersResponse>("/api/admin/users");
      return response;
    });
    if (result) {
      setUsers(result.users);
      setLoadState("success");
    } else {
      setLoadState("error");
    }
  }, [toastCall]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleAssume = useCallback(
    async (userId: string) => {
      setAssumingUserId(userId);
      try {
        await onAssumeIdentity(userId);
      } catch {
        setAssumingUserId(null);
      }
    },
    [onAssumeIdentity],
  );

  return {
    state: {
      loadState,
      users,
      assumingUserId,
      deletion: deletionState,
    },
    actions: {
      handleAssume,
      retry: fetchUsers,
      deletion: deletionActions,
    },
  };
}
