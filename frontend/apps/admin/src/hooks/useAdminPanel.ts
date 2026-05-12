import { useState, useEffect, useCallback } from "react";
import { apiClient, useApiToast } from "@gofin/api";
import { useUserDeletion } from "./useUserDeletion";
import type { UserDeletionState, UserDeletionActions } from "./useUserDeletion";
import type { AdminUser, AdminUsersResponse } from "../types";
import type { User } from "@gofin/core";

type AdminLoadState = "loading" | "error" | "success";

export interface AdminPanelState {
  /** Page load state. */
  loadState: AdminLoadState;
  /** List of registered users. */
  users: AdminUser[];
  /** User ID currently being assumed (for loading indicator). */
  assumingUserId: string | null;
  /** Deletion workflow state. */
  deletion: UserDeletionState;
}

export interface AdminPanelActions {
  /** Assume identity of a user. */
  handleAssume: (userId: string) => void;
  /** Retry fetching users after error. */
  retry: () => void;
  /** Deletion workflow actions. */
  deletion: UserDeletionActions;
}

export interface UseAdminPanelOptions {
  /** Current authenticated admin user. */
  currentUser: User | null;
  /** Callback to execute identity assumption. */
  onAssumeIdentity: (userId: string) => Promise<void>;
}

export function useAdminPanel(options: UseAdminPanelOptions): {
  state: AdminPanelState;
  actions: AdminPanelActions;
} {
  const { onAssumeIdentity } = options;
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loadState, setLoadState] = useState<AdminLoadState>("loading");
  const [assumingUserId, setAssumingUserId] = useState<string | null>(null);
  const { call: toastCall } = useApiToast();

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
