import { useState, useCallback, type FormEvent } from "react";
import { useFormMutation } from "@gofin/api";
import type { User } from "@gofin/core";
import { settingsApi } from "../api";

export interface ProfileFormState {
  username: string;
  email: string;
  error: string | null;
  success: boolean;
  loading: boolean;
}

export interface ProfileFormActions {
  setUsername: (value: string) => void;
  setEmail: (value: string) => void;
  handleSubmit: (event: FormEvent) => void;
}

export function useProfileForm(
  user: User,
  onUserUpdated?: () => void,
): { state: ProfileFormState; actions: ProfileFormActions } {
  const [username, setUsername] = useState(user.username);
  const [email, setEmail] = useState(user.email);
  const [success, setSuccess] = useState(false);

  const { submit, error: mutationError, submitting } = useFormMutation<void>({
    onSuccess: () => {
      onUserUpdated?.();
      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    },
  });

  const handleSubmit = useCallback(
    (event: FormEvent) => {
      event.preventDefault();
      setSuccess(false);

      submit(() =>
        settingsApi.updateProfile({
          username: username.trim(),
          email: email.trim(),
          currency: user.currency,
        }),
      );
    },
    [username, email, user.currency, submit],
  );

  return {
    state: { username, email, error: mutationError, success, loading: submitting },
    actions: { setUsername, setEmail, handleSubmit },
  };
}
