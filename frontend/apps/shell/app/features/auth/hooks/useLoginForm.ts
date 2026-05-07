import { useState, useEffect, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { consumeReturnToPath, useFormMutation } from "@gofin/api";
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
  const [validationError, setValidationError] = useState<string | null>(null);

  const isSessionExpired = searchParams.get("expired") === "true";

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate("/dashboard");
    }
  }, [isLoading, isAuthenticated, navigate]);

  const mutation = useFormMutation<Awaited<ReturnType<typeof login>>>({
    onSuccess: (user) => {
      const returnTo = consumeReturnToPath();

      if (!user.hasCompletedOnboarding) {
        navigate("/onboarding");
      } else if (returnTo && returnTo !== "/login" && returnTo !== "/register") {
        navigate(returnTo);
      } else {
        navigate("/dashboard");
      }
    },
  });

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    setValidationError(null);

    const emailError = validateEmail(email);
    if (emailError) {
      setValidationError(emailError);
      return;
    }
    if (!password) {
      setValidationError("Password is required");
      return;
    }

    mutation.submit(() => login(email, password));
  };

  return {
    email,
    password,
    setEmail,
    setPassword,
    error: validationError || mutation.error,
    submitting: mutation.submitting,
    isSessionExpired,
    handleSubmit,
  };
}
