import { useState, useEffect, useCallback, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { ApiRequestError, useFormMutation } from "@gofin/api";
import {
  validateEmail,
  validatePassword,
  validateUsername,
} from "@gofin/core";

/** Grouped registration form fields. */
export interface RegisterFields {
  username: string;
  email: string;
  password: string;
  confirmPassword: string;
}

export interface RegisterFormState {
  fields: RegisterFields;
  errors: Record<string, string>;
  submitting: boolean;
}

export interface RegisterFormActions {
  setField: (key: keyof RegisterFields, value: string) => void;
  handleSubmit: (event: FormEvent) => void;
}

const INITIAL_FIELDS: RegisterFields = {
  username: "",
  email: "",
  password: "",
  confirmPassword: "",
};

export function useRegisterForm(): { state: RegisterFormState; actions: RegisterFormActions } {
  const { isAuthenticated, isLoading, checkAuth, register } = useAuthStore();
  const navigate = useNavigate();

  const [fields, setFields] = useState<RegisterFields>(INITIAL_FIELDS);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate("/dashboard");
    }
  }, [isLoading, isAuthenticated, navigate]);

  const setField = useCallback(
    (key: keyof RegisterFields, value: string) => {
      setFields((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const mutation = useFormMutation<void>({
    onError: (errorMessage) => setErrors((prev) => ({ ...prev, form: errorMessage })),
  });

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    setErrors({});

    const validationErrors: Record<string, string> = {};

    const usernameError = validateUsername(fields.username);
    if (usernameError) validationErrors.username = usernameError;

    const emailError = validateEmail(fields.email);
    if (emailError) validationErrors.email = emailError;

    const passwordError = validatePassword(fields.password);
    if (passwordError) validationErrors.password = passwordError;

    if (fields.confirmPassword && fields.confirmPassword !== fields.password) {
      validationErrors.confirmPassword = "Passwords do not match";
    }

    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }

    mutation.submit(async () => {
      try {
        await register(fields.username, fields.email, fields.password);
        navigate("/onboarding");
      } catch (err) {
        if (err instanceof ApiRequestError) {
          if (err.code === "DUPLICATE_EMAIL") {
            setErrors({ email: err.message });
            return;
          }
          if (err.code === "DUPLICATE_USERNAME") {
            setErrors({ username: err.message });
            return;
          }
          if (err.fields) {
            setErrors(err.fields);
            return;
          }
        }
        throw err;
      }
    });
  };

  return {
    state: {
      fields,
      errors,
      submitting: mutation.submitting,
    },
    actions: {
      setField,
      handleSubmit,
    },
  };
}
