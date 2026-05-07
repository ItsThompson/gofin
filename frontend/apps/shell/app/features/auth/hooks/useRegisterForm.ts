import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { ApiRequestError, useFormMutation } from "@gofin/api";
import {
  validateEmail,
  validatePassword,
  validateUsername,
} from "@/lib/validation";

export interface RegisterFormResult {
  username: string;
  email: string;
  password: string;
  confirmPassword: string;
  setUsername: (value: string) => void;
  setEmail: (value: string) => void;
  setPassword: (value: string) => void;
  setConfirmPassword: (value: string) => void;
  errors: Record<string, string>;
  submitting: boolean;
  handleSubmit: (event: FormEvent) => void;
}

export function useRegisterForm(): RegisterFormResult {
  const { isAuthenticated, isLoading, checkAuth, register } = useAuthStore();
  const navigate = useNavigate();

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate("/dashboard");
    }
  }, [isLoading, isAuthenticated, navigate]);

  const mutation = useFormMutation<void>({
    onError: (errorMessage) => setErrors((prev) => ({ ...prev, form: errorMessage })),
  });

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    setErrors({});

    const validationErrors: Record<string, string> = {};

    const usernameError = validateUsername(username);
    if (usernameError) validationErrors.username = usernameError;

    const emailError = validateEmail(email);
    if (emailError) validationErrors.email = emailError;

    const passwordError = validatePassword(password);
    if (passwordError) validationErrors.password = passwordError;

    if (confirmPassword && confirmPassword !== password) {
      validationErrors.confirmPassword = "Passwords do not match";
    }

    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }

    mutation.submit(async () => {
      try {
        await register(username, email, password);
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
    username,
    email,
    password,
    confirmPassword,
    setUsername,
    setEmail,
    setPassword,
    setConfirmPassword,
    errors,
    submitting: mutation.submitting,
    handleSubmit,
  };
}
