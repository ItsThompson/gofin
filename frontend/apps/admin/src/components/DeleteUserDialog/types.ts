export interface DeleteUserDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: { id: string; username: string } | null;
  onSuccess: () => void;
}

export interface PasswordStepProps {
  userId: string;
  username: string;
  onSuccess: () => void;
  onOpenChange: (open: boolean) => void;
}
