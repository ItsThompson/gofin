import type { User } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import { Check, Loader2 } from "lucide-react";
import { useProfileForm } from "../hooks/useProfileForm";

export function ProfileSection({ user, onUserUpdated }: { user: User; onUserUpdated?: () => void }) {
  const { state, actions } = useProfileForm(user, onUserUpdated);

  return (
    <Form onSubmit={actions.handleSubmit}>
      <FormField>
        <FormLabel htmlFor="profile-username">Username</FormLabel>
        <Input
          id="profile-username"
          type="text"
          value={state.username}
          onChange={(event) => actions.setUsername(event.target.value)}
          required
        />
      </FormField>

      <FormField>
        <FormLabel htmlFor="profile-email">Email</FormLabel>
        <Input
          id="profile-email"
          type="email"
          value={state.email}
          onChange={(event) => actions.setEmail(event.target.value)}
          required
        />
      </FormField>

      <FormMessage>{state.error}</FormMessage>

      {state.success && (
        <p className="flex items-center gap-1.5 text-sm text-green-600">
          <Check className="size-4" />
          Profile updated successfully.
        </p>
      )}

      <Button type="submit" disabled={state.loading}>
        {state.loading && <Loader2 className="size-4 animate-spin" />}
        Update Profile
      </Button>
    </Form>
  );
}
