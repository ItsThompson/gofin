import { useState, useEffect, useCallback, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { consumeReturnToPath, useFormMutation } from "@gofin/api";
import { getLandingPath, validateEmail } from "@gofin/core";

/** Grouped login credential fields. */
export interface LoginCredentials {
  email: string;
  password: string;
}

export interface LoginFormState {
  credentials: LoginCredentials;
  error: string | null;
  submitting: boolean;
  isSessionExpired: boolean;
}

export interface LoginFormActions {
  setField: (key: keyof LoginCredentials, value: string) => void;
  handleSubmit: (event: FormEvent) => void;
}

const INITIAL_CREDENTIALS: LoginCredentials = {
  email: "",
  password: "",
};

export function useLoginForm(): { state: LoginFormState; actions: LoginFormActions } {
  const { login, isAuthenticated, isLoading, checkAuth, user } = useAuthStore();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const [credentials, setCredentials] = useState<LoginCredentials>(INITIAL_CREDENTIALS);
  const [validationError, setValidationError] = useState<string | null>(null);

  const isSessionExpired = searchParams.get("expired") === "true";

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && isAuthenticated && user) {
      navigate(getLandingPath(user));
    }
  }, [isLoading, isAuthenticated, user, navigate]);

  const setField = useCallback(
    (key: keyof LoginCredentials, value: string) => {
      setCredentials((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const mutation = useFormMutation<Awaited<ReturnType<typeof login>>>({
    onSuccess: (loggedInUser) => {
      const returnTo = consumeReturnToPath();

      if (!loggedInUser.hasCompletedOnboarding) {
        navigate("/onboarding");
      } else if (returnTo && returnTo !== "/login" && returnTo !== "/register") {
        navigate(returnTo);
      } else {
        navigate(getLandingPath(loggedInUser));
      }
    },
  });

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    setValidationError(null);

    const emailError = validateEmail(credentials.email);
    if (emailError) {
      setValidationError(emailError);
      return;
    }
    if (!credentials.password) {
      setValidationError("Password is required");
      return;
    }

    mutation.submit(() => login(credentials.email, credentials.password));
  };

  return {
    state: {
      credentials,
      error: validationError || mutation.error,
      submitting: mutation.submitting,
      isSessionExpired,
    },
    actions: {
      setField,
      handleSubmit,
    },
  };
}
