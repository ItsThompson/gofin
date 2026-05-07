import { useState, useCallback, type FormEvent } from "react";
import { ApiRequestError } from "@gofin/api";
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
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      setError(null);
      setSuccess(false);

      // Client-side validation
      const strengthError = validatePasswordStrength(newPassword);
      if (strengthError) {
        setError(strengthError);
        return;
      }

      setLoading(true);

      try {
        await settingsApi.changePassword({
          currentPassword,
          newPassword,
        });

        setCurrentPassword("");
        setNewPassword("");
        onUserUpdated?.();
        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        if (err instanceof ApiRequestError) {
          if (err.code === "INVALID_CREDENTIALS") {
            setError("Current password is incorrect.");
          } else if (err.code === "WEAK_PASSWORD") {
            setError(err.message);
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
    [currentPassword, newPassword, onUserUpdated],
  );

  return {
    state: { currentPassword, newPassword, error, success, loading },
    actions: { setCurrentPassword, setNewPassword, handleSubmit },
  };
}
