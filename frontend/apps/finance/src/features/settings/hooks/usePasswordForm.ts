import { useState, useCallback, type FormEvent } from "react";
import { useFormMutation } from "@gofin/api";
import { settingsApi } from "../api";

/**
 * Validates password strength: 8+ chars, 1 upper, 1 lower, 1 digit.
 */
function validatePasswordStrength(password: string): string | null {
  if (password.length < 8) {
    return "Password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit";
  }
  if (!/[A-Z]/.test(password)) {
    return "Password must contain at least one uppercase letter";
  }
  if (!/[a-z]/.test(password)) {
    return "Password must contain at least one lowercase letter";
  }
  if (!/\d/.test(password)) {
    return "Password must contain at least one digit";
  }
  return null;
}

export interface PasswordFormState {
  currentPassword: string;
  newPassword: string;
  error: string | null;
  success: boolean;
  loading: boolean;
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
  const [success, setSuccess] = useState(false);

  const { submit, error: mutationError, submitting } = useFormMutation<void>({
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
      onUserUpdated?.();
      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    },
  });

  const handleSubmit = useCallback(
    (event: FormEvent) => {
      event.preventDefault();
      setValidationError(null);
      setSuccess(false);

      // Client-side validation
      const strengthError = validatePasswordStrength(newPassword);
      if (strengthError) {
        setValidationError(strengthError);
        return;
      }

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
      error: validationError || mutationError,
      success,
      loading: submitting,
    },
    actions: { setCurrentPassword, setNewPassword, handleSubmit },
  };
}
