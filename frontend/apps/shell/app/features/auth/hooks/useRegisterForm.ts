import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { ApiRequestError } from "@gofin/api";
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
  const [submitting, setSubmitting] = useState(false);

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

    setSubmitting(true);
    try {
      await register(username, email, password);
      navigate("/onboarding");
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.code === "DUPLICATE_EMAIL") {
          setErrors({ email: err.message });
        } else if (err.code === "DUPLICATE_USERNAME") {
          setErrors({ username: err.message });
        } else if (err.fields) {
          setErrors(err.fields);
        } else {
          setErrors({ form: err.message });
        }
      } else {
        setErrors({ form: "An unexpected error occurred. Please try again." });
      }
    } finally {
      setSubmitting(false);
    }
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
    submitting,
    handleSubmit,
  };
}
