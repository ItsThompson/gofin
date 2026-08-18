import { useState, useCallback, type FormEvent } from "react";
import { useFormMutation } from "@gofin/api";
import { validatePassword } from "@gofin/core";
import { settingsApi, type AuthResponse } from "../api";
import type { SaveStatus } from "../types";

export interface PasswordFormState {
  currentPassword: string;
  newPassword: string;
  /** Client-side password strength error, or null when the password is valid. */
  validationError: string | null;
  /** Single status for the save operation; failure message travels with `failed`. */
  saveStatus: SaveStatus;
}

export interface PasswordFormActions {
  setCurrentPassword: (value: string) => void;
  setNewPassword: (value: string) => void;
  handleSubmit: (event: FormEvent) => void;
}

export function usePasswordForm(
  onUserUpdated?: () => void,
): { state: PasswordFormState; actions: PasswordFormActions } {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>({ kind: "idle" });

  const { submit } = useFormMutation<AuthResponse>({
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
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
      setValidationError(null);
      setSaveStatus({ kind: "idle" });

      // Client-side validation
      const strengthError = validatePassword(newPassword);
      if (strengthError) {
        setValidationError(strengthError);
        return;
      }

      setSaveStatus({ kind: "saving" });

      submit(() =>
        settingsApi.changePassword({
          currentPassword,
          newPassword,
        }),
      );
    },
    [currentPassword, newPassword, submit],
  );

  return {
    state: {
      currentPassword,
      newPassword,
      validationError,
      saveStatus,
    },
    actions: { setCurrentPassword, setNewPassword, handleSubmit },
  };
}
