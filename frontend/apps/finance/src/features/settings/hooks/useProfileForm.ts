import { useState, useCallback, type FormEvent } from "react";
import { useFormMutation } from "@gofin/api";
import type { User } from "@gofin/core";
import { settingsApi, type AuthResponse } from "../api";
import type { SaveStatus } from "../types";

export interface ProfileFormState {
  username: string;
  email: string;
  /** Single status for the save operation; failure message travels with `failed`. */
  saveStatus: SaveStatus;
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
  const [saveStatus, setSaveStatus] = useState<SaveStatus>({ kind: "idle" });

  const { submit } = useFormMutation<AuthResponse>({
    onSuccess: () => {
      onUserUpdated?.();
      setSaveStatus({ kind: "saved" });
      setTimeout(() => setSaveStatus({ kind: "idle" }), 3000);
    },
    onError: (message) => {
      setSaveStatus({ kind: "failed", message });
    },
  });

  const handleSubmit = useCallback(
    (event: FormEvent) => {
      event.preventDefault();
      setSaveStatus({ kind: "saving" });

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
    state: { username, email, saveStatus },
    actions: { setUsername, setEmail, handleSubmit },
  };
}
