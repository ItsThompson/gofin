import { Link } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
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
import { useRegisterForm } from "../hooks/useRegisterForm";

export function RegisterPage() {
  const { isLoading } = useAuthStore();
  const {
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
  } = useRegisterForm();

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
          <CardTitle className="text-2xl">Create your account</CardTitle>
          <CardDescription>
            Get started with GoFin to track your finances
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form onSubmit={handleSubmit}>
            <FormField>
              <FormLabel htmlFor="username">Username</FormLabel>
              <Input
                id="username"
                type="text"
                placeholder="johndoe"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                autoComplete="username"
                aria-invalid={!!errors.username}
                required
              />
              <FormMessage>{errors.username}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="email">Email</FormLabel>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                autoComplete="email"
                aria-invalid={!!errors.email}
                required
              />
              <FormMessage>{errors.email}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="password">Password</FormLabel>
              <Input
                id="password"
                type="password"
                placeholder="At least 8 characters"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete="new-password"
                aria-invalid={!!errors.password}
                required
              />
              <FormMessage>{errors.password}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="confirmPassword">Confirm Password</FormLabel>
              <Input
                id="confirmPassword"
                type="password"
                placeholder="Re-enter your password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                autoComplete="new-password"
                aria-invalid={!!errors.confirmPassword}
                required
              />
              <FormMessage>{errors.confirmPassword}</FormMessage>
            </FormField>

            {errors.form && <FormMessage>{errors.form}</FormMessage>}

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "Creating account..." : "Create account"}
            </Button>

            <p className="text-center text-sm text-muted-foreground">
              Already have an account?{" "}
              <Link
                to="/login"
                className="font-medium text-primary underline-offset-4 hover:underline"
              >
                Sign in
              </Link>
            </p>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
