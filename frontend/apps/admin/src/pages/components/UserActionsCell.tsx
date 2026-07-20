import { Button } from "@gofin/ui/components/button";
import { Loader2, UserCheck, Trash2 } from "lucide-react";
import type { AdminUserSummary } from "@gofin/core";
import type { DeletionStatus } from "../../components/DeleteUserDialog/types";

const PROTECTED_USERNAMES = ["admin", "thompson"];

function isProtectedUser(username: string): boolean {
  return PROTECTED_USERNAMES.includes(username);
}

interface UserActionsCellProps {
  user: AdminUserSummary;
  deletionState: { jobId: string; status: DeletionStatus; error?: string } | undefined;
  assumingUserId: string | null;
  onAssume: (userId: string) => void;
  onDelete: (user: AdminUserSummary) => void;
}

export function UserActionsCell({
  user,
  deletionState,
  assumingUserId,
  onAssume,
  onDelete,
}: UserActionsCellProps) {
  const status = deletionState?.status;

  if (status === "pending" || status === "running") {
    return (
      <div className="flex items-center gap-2">
        <Loader2 className="size-3 animate-spin" />
        <span className="text-sm text-muted-foreground">Deleting...</span>
      </div>
    );
  }

  if (status === "failed") {
    return (
      <div className="flex items-center gap-1">
        <span className="inline-flex items-center rounded-full bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
          Failed
        </span>
        {!isProtectedUser(user.username) && (
          <Button
            variant="destructive"
            size="icon-sm"
            onClick={() => onDelete(user)}
            aria-label={`Delete ${user.username}`}
          >
            <Trash2 className="size-3" />
          </Button>
        )}
      </div>
    );
  }

  // idle or no deletion state
  return (
    <div className="flex items-center gap-1">
      <Button
        variant="outline"
        size="sm"
        onClick={() => onAssume(user.id)}
        disabled={assumingUserId !== null}
      >
        {assumingUserId === user.id ? (
          <Loader2 className="size-3 animate-spin" />
        ) : (
          <UserCheck className="size-3" />
        )}
        Assume
      </Button>
      {!isProtectedUser(user.username) && (
        <Button
          variant="destructive"
          size="icon-sm"
          onClick={() => onDelete(user)}
          aria-label={`Delete ${user.username}`}
        >
          <Trash2 className="size-3" />
        </Button>
      )}
    </div>
  );
}
