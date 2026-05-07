import { useState, useCallback, type FormEvent } from "react";
import { ApiRequestError } from "@gofin/api";
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
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      setError(null);
      setSuccess(false);
      setLoading(true);

      try {
        await settingsApi.updateProfile({
          username: username.trim(),
          email: email.trim(),
          currency: user.currency,
        });

        onUserUpdated?.();
        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        if (err instanceof ApiRequestError) {
          if (err.code === "DUPLICATE_EMAIL") {
            setError("An account with this email already exists.");
          } else if (err.code === "DUPLICATE_USERNAME") {
            setError("This username is already taken.");
          } else {
            setError(err.message);
          }
        } else {
          setError("An unexpected error occurred. Please try again.");
        }
      } finally {
        setLoading(false);
      }
    },
    [username, email, user.currency, onUserUpdated],
  );

  return {
    state: { username, email, error, success, loading },
    actions: { setUsername, setEmail, handleSubmit },
  };
}
