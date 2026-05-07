import { useState, useCallback } from "react";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import { Label } from "@gofin/ui/components/label";
import {
  MultiStepDialog,
  MultiStepDialogContent,
  MultiStepDialogStep,
  useMultiStepDialog,
} from "@gofin/ui/components/multi-step-dialog";
import { DialogHeader, DialogTitle, DialogClose } from "@gofin/ui/components/dialog";
import { apiClient, ApiRequestError } from "@gofin/api";
import { Loader2 } from "lucide-react";

interface DeleteUserDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: { id: string; username: string } | null;
  onSuccess: () => void;
}

const CONFIRMATION_PHRASE = "permanently delete";

function ConfirmationStep() {
  const { next } = useMultiStepDialog();
  const [confirmText, setConfirmText] = useState("");

  const isValid = confirmText === CONFIRMATION_PHRASE;

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        This action is permanent and cannot be undone. To confirm, type{" "}
        <span className="font-mono font-semibold text-destructive">{CONFIRMATION_PHRASE}</span>{" "}
        below.
      </p>
      <div className="space-y-2">
        <Label htmlFor="confirm-phrase">Confirmation</Label>
        <Input
          id="confirm-phrase"
          value={confirmText}
          onChange={(event) => setConfirmText(event.target.value)}
          placeholder={CONFIRMATION_PHRASE}
          autoComplete="off"
        />
      </div>
      <div className="flex justify-end">
        <Button onClick={next} disabled={!isValid}>
          Next
        </Button>
      </div>
    </div>
  );
}

interface PasswordStepProps {
  userId: string;
  username: string;
  onSuccess: () => void;
  onOpenChange: (open: boolean) => void;
}

function PasswordStep({ userId, username, onSuccess, onOpenChange }: PasswordStepProps) {
  const { back } = useMultiStepDialog();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    setError("");
    setIsSubmitting(true);

    try {
      await apiClient(`/api/admin/users/${userId}`, {
        method: "DELETE",
        body: JSON.stringify({ password }),
      });
      onOpenChange(false);
      onSuccess();
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred");
      }
    } finally {
      setIsSubmitting(false);
    }
  }, [userId, password, onSuccess, onOpenChange]);

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Enter your admin password to confirm deletion of user{" "}
        <span className="font-semibold">{username}</span>.
      </p>
      <div className="space-y-2">
        <Label htmlFor="admin-password">Your Password</Label>
        <Input
          id="admin-password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder="Enter your password"
          autoComplete="current-password"
        />
        {error && (
          <p className="text-sm text-destructive">{error}</p>
        )}
      </div>
      <div className="flex justify-between">
        <Button variant="outline" onClick={back} disabled={isSubmitting}>
          Back
        </Button>
        <Button
          variant="destructive"
          onClick={handleSubmit}
          disabled={!password || isSubmitting}
        >
          {isSubmitting && <Loader2 className="size-3 animate-spin" />}
          Delete User
        </Button>
      </div>
    </div>
  );
}

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
