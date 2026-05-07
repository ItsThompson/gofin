import { useState, useEffect, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { ApiRequestError, consumeReturnToPath } from "@gofin/api";
import { validateEmail } from "@/lib/validation";

export interface LoginFormResult {
  email: string;
  password: string;
  setEmail: (value: string) => void;
  setPassword: (value: string) => void;
  error: string | null;
  submitting: boolean;
  isSessionExpired: boolean;
  handleSubmit: (event: FormEvent) => void;
}

export function useLoginForm(): LoginFormResult {
  const { login, isAuthenticated, isLoading, checkAuth } = useAuthStore();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const isSessionExpired = searchParams.get("expired") === "true";

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate("/dashboard");
    }
  }, [isLoading, isAuthenticated, navigate]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);

    const emailError = validateEmail(email);
    if (emailError) {
      setError(emailError);
      return;
    }
    if (!password) {
      setError("Password is required");
      return;
    }

    setSubmitting(true);
    try {
      const user = await login(email, password);

      const returnTo = consumeReturnToPath();

      if (!user.hasCompletedOnboarding) {
        navigate("/onboarding");
      } else if (returnTo && returnTo !== "/login" && returnTo !== "/register") {
        navigate(returnTo);
      } else {
        navigate("/dashboard");
      }
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred. Please try again.");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return {
    email,
    password,
    setEmail,
    setPassword,
    error,
    submitting,
    isSessionExpired,
    handleSubmit,
  };
}
