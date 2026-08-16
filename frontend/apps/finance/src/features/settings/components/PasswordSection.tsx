import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import { Check, Loader2 } from "lucide-react";
import { usePasswordForm } from "../hooks/usePasswordForm";

export function PasswordSection({ onUserUpdated }: { onUserUpdated?: () => void }) {
  const { state, actions } = usePasswordForm(onUserUpdated);

  const saveError =
    state.saveStatus.kind === "failed" ? state.saveStatus.message : null;

  return (
    <Form onSubmit={actions.handleSubmit}>
      <FormField>
        <FormLabel htmlFor="current-password">Current Password</FormLabel>
        <Input
          id="current-password"
          type="password"
          value={state.currentPassword}
          onChange={(event) => actions.setCurrentPassword(event.target.value)}
          required
        />
      </FormField>

      <FormField>
        <FormLabel htmlFor="new-password">New Password</FormLabel>
        <Input
          id="new-password"
          type="password"
          value={state.newPassword}
          onChange={(event) => actions.setNewPassword(event.target.value)}
          required
        />
        <p className="text-xs text-muted-foreground">
          Minimum 8 characters with at least one uppercase letter, one lowercase letter, and one digit.
        </p>
      </FormField>

      <FormMessage>{state.validationError || saveError}</FormMessage>

      {state.saveStatus.kind === "saved" && (
        <p className="flex items-center gap-1.5 text-sm text-green-600">
          <Check className="size-4" />
          Password changed successfully. Other sessions have been signed out.
        </p>
      )}

      <Button type="submit" disabled={state.saveStatus.kind === "saving"}>
        {state.saveStatus.kind === "saving" && (
          <Loader2 className="size-4 animate-spin" />
        )}
        Change Password
      </Button>
    </Form>
  );
}
