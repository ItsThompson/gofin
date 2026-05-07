import {
  MultiStepDialog,
  MultiStepDialogContent,
  MultiStepDialogStep,
} from "@gofin/ui/components/multi-step-dialog";
import { DialogHeader, DialogTitle, DialogClose } from "@gofin/ui/components/dialog";
import { ConfirmationStep } from "./components/ConfirmationStep";
import { PasswordStep } from "./components/PasswordStep";
import type { DeleteUserDialogProps } from "./types";

/**
 * Two-step delete user confirmation dialog.
 * Step 1: Type "permanently delete" to proceed.
 * Step 2: Enter admin password to confirm.
 */
export function DeleteUserDialog({ open, onOpenChange, user, onSuccess }: DeleteUserDialogProps) {
  if (!user) return null;

  return (
    <MultiStepDialog open={open} onOpenChange={onOpenChange}>
      <MultiStepDialogContent>
        <DialogHeader>
          <DialogTitle>Delete User: {user.username}</DialogTitle>
          <DialogClose onClick={() => onOpenChange(false)} />
        </DialogHeader>
        <MultiStepDialogStep>
          <ConfirmationStep />
        </MultiStepDialogStep>
        <MultiStepDialogStep>
          <PasswordStep
            userId={user.id}
            username={user.username}
            onSuccess={onSuccess}
            onOpenChange={onOpenChange}
          />
        </MultiStepDialogStep>
      </MultiStepDialogContent>
    </MultiStepDialog>
  );
}
