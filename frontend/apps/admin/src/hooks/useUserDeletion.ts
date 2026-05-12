import { useState, useCallback } from "react";
import { toast } from "sonner";
import { useDeletionPolling } from "./useDeletionPolling";
import type { DeletionJobResponse, DeletionStateMap, DeletionStatus } from "../components/DeleteUserDialog/types";

export interface UserDeletionState {
  /** User currently targeted for deletion confirmation dialog, or null. */
  deletingUser: { id: string; username: string } | null;
  /** Map of user IDs to their deletion status (for inline status display). */
  deletionStates: DeletionStateMap;
  /** Whether a deletion job is actively being polled. */
  isPolling: boolean;
}

export interface UserDeletionActions {
  /** Open the deletion confirmation dialog for a user. */
  startDeletion: (user: { id: string; username: string }) => void;
  /** Close the deletion confirmation dialog without deleting. */
  cancelDeletion: () => void;
  /** Called when the deletion API returns a job. Starts polling. */
  handleDeletionSuccess: (job: DeletionJobResponse) => void;
}

export interface UseUserDeletionOptions {
  /** Called when a deletion job completes successfully. Receives the deleted user ID. */
  onUserRemoved: (userId: string) => void;
}

export function useUserDeletion(options: UseUserDeletionOptions): {
  state: UserDeletionState;
  actions: UserDeletionActions;
} {
  const [deletingUser, setDeletingUser] = useState<{ id: string; username: string } | null>(null);
  const [deletionStates, setDeletionStates] = useState<DeletionStateMap>({});
  const [activePolling, setActivePolling] = useState<{ jobId: string; userId: string; username: string } | null>(null);

  const handleDeletionSuccess = useCallback((job: DeletionJobResponse) => {
    const username = deletingUser?.username ?? "";
    setDeletionStates((prev) => ({
      ...prev,
      [job.userId]: { jobId: job.id, status: "pending" },
    }));
    setActivePolling({ jobId: job.id, userId: job.userId, username });
    setDeletingUser(null);
  }, [deletingUser]);

  const handleStatusChange = useCallback((status: DeletionStatus, error?: string) => {
    if (!activePolling) return;
    setDeletionStates((prev) => ({
      ...prev,
      [activePolling.userId]: { jobId: activePolling.jobId, status, error },
    }));
  }, [activePolling]);

  const handlePollingCompleted = useCallback(() => {
    if (!activePolling) return;
    const { userId, username } = activePolling;
    setDeletionStates((prev) => {
      const next = { ...prev };
      delete next[userId];
      return next;
    });
    setActivePolling(null);
    toast.success(`User "${username}" has been deleted`);
    options.onUserRemoved(userId);
  }, [activePolling, options]);

  const handlePollingFailed = useCallback((error: string) => {
    if (!activePolling) return;
    const { username } = activePolling;
    setActivePolling(null);
    toast.error(`Deletion of "${username}" failed: ${error}`);
  }, [activePolling]);

  useDeletionPolling({
    jobId: activePolling?.jobId ?? "",
    enabled: activePolling !== null,
    onStatusChange: handleStatusChange,
    onCompleted: handlePollingCompleted,
    onFailed: handlePollingFailed,
  });

  const startDeletion = useCallback((user: { id: string; username: string }) => {
    setDeletingUser(user);
  }, []);

  const cancelDeletion = useCallback(() => {
    setDeletingUser(null);
  }, []);

  return {
    state: {
      deletingUser,
      deletionStates,
      isPolling: activePolling !== null,
    },
    actions: {
      startDeletion,
      cancelDeletion,
      handleDeletionSuccess,
    },
  };
}
