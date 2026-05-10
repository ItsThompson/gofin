export type DeletionStatus = "idle" | "pending" | "running" | "failed" | "completed";

export interface DeletionJobResponse {
  id: string;
  userId: string;
  status: DeletionStatus;
  error: string | null;
  createdAt: string;
  completedAt: string | null;
}

export interface UserDeletionState {
  jobId: string;
  status: DeletionStatus;
  error?: string;
}

export type DeletionStateMap = Record<string, UserDeletionState>;

export interface DeleteUserDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: { id: string; username: string } | null;
  onSuccess: (job: DeletionJobResponse) => void;
}

export interface PasswordStepProps {
  userId: string;
  username: string;
  onSuccess: (job: DeletionJobResponse) => void;
  onOpenChange: (open: boolean) => void;
}
