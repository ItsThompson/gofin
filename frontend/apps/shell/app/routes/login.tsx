import { useState, useEffect, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { ApiRequestError, consumeReturnToPath } from "@gofin/api";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import { validateEmail } from "@/lib/validation";

export default function LoginPage() {
  const { login, isAuthenticated, isLoading, checkAuth } = useAuthStore();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<{ form?: string }>({});
  const [submitting, setSubmitting] = useState(false);

  const isSessionExpired = searchParams.get("expired") === "true";

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  // Redirect to dashboard if already authenticated
  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate("/dashboard");
    }
  }, [isLoading, isAuthenticated, navigate]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setErrors({});

    // Client-side validation
    const emailError = validateEmail(email);
    if (emailError) {
      setErrors({ form: emailError });
      return;
    }
    if (!password) {
      setErrors({ form: "Password is required" });
      return;
    }

    setSubmitting(true);
    try {
      const user = await login(email, password);

      // Check for a saved return-to path (from session expiry redirect)
      const returnTo = consumeReturnToPath();

      if (!user.hasCompletedOnboarding) {
        navigate("/onboarding");
      } else if (returnTo && returnTo !== "/login" && returnTo !== "/register") {
        navigate(returnTo);
      } else {
        navigate("/dashboard");
      }
    } catch (error) {
      if (error instanceof ApiRequestError) {
        // Generic error message for invalid credentials (no field hint per spec)
        setErrors({ form: error.message });
      } else {
        setErrors({ form: "An unexpected error occurred. Please try again." });
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl">Sign in to GoFin</CardTitle>
          <CardDescription>
            Enter your email and password to access your account
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isSessionExpired && (
            <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">
              Your session has expired. Please log in again.
            </div>
          )}
          <Form onSubmit={handleSubmit}>
            <FormField>
              <FormLabel htmlFor="email">Email</FormLabel>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                autoComplete="email"
                required
              />
            </FormField>

            <FormField>
              <FormLabel htmlFor="password">Password</FormLabel>
              <Input
                id="password"
                type="password"
                placeholder="Enter your password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete="current-password"
                required
              />
            </FormField>

            {errors.form && <FormMessage>{errors.form}</FormMessage>}

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "Signing in..." : "Sign in"}
            </Button>

            <p className="text-center text-sm text-muted-foreground">
              Don&apos;t have an account?{" "}
              <Link
                to="/register"
                className="font-medium text-primary underline-offset-4 hover:underline"
              >
                Create one
              </Link>
            </p>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
