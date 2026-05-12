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
  const { state, actions } = useRegisterForm();

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
          <Form onSubmit={actions.handleSubmit}>
            <FormField>
              <FormLabel htmlFor="username">Username</FormLabel>
              <Input
                id="username"
                type="text"
                placeholder="johndoe"
                value={state.fields.username}
                onChange={(event) => actions.setField("username", event.target.value)}
                autoComplete="username"
                aria-invalid={!!state.errors.username}
                required
              />
              <FormMessage>{state.errors.username}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="email">Email</FormLabel>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                value={state.fields.email}
                onChange={(event) => actions.setField("email", event.target.value)}
                autoComplete="email"
                aria-invalid={!!state.errors.email}
                required
              />
              <FormMessage>{state.errors.email}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="password">Password</FormLabel>
              <Input
                id="password"
                type="password"
                placeholder="At least 8 characters"
                value={state.fields.password}
                onChange={(event) => actions.setField("password", event.target.value)}
                autoComplete="new-password"
                aria-invalid={!!state.errors.password}
                required
              />
              <FormMessage>{state.errors.password}</FormMessage>
            </FormField>

            <FormField>
              <FormLabel htmlFor="confirmPassword">Confirm Password</FormLabel>
              <Input
                id="confirmPassword"
                type="password"
                placeholder="Re-enter your password"
                value={state.fields.confirmPassword}
                onChange={(event) => actions.setField("confirmPassword", event.target.value)}
                autoComplete="new-password"
                aria-invalid={!!state.errors.confirmPassword}
                required
              />
              <FormMessage>{state.errors.confirmPassword}</FormMessage>
            </FormField>

            {state.errors.form && <FormMessage>{state.errors.form}</FormMessage>}

            <Button type="submit" className="w-full" disabled={state.submitting}>
              {state.submitting ? "Creating account..." : "Create account"}
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
