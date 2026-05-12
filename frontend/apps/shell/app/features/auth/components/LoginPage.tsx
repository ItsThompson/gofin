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
import { useLoginForm } from "../hooks/useLoginForm";

export function LoginPage() {
  const { isLoading } = useAuthStore();
  const { state, actions } = useLoginForm();

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
          {state.isSessionExpired && (
            <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">
              Your session has expired. Please log in again.
            </div>
          )}
          <Form onSubmit={actions.handleSubmit}>
            <FormField>
              <FormLabel htmlFor="email">Email</FormLabel>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                value={state.credentials.email}
                onChange={(event) => actions.setField("email", event.target.value)}
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
                value={state.credentials.password}
                onChange={(event) => actions.setField("password", event.target.value)}
                autoComplete="current-password"
                required
              />
            </FormField>

            {state.error && <FormMessage>{state.error}</FormMessage>}

            <Button type="submit" className="w-full" disabled={state.submitting}>
              {state.submitting ? "Signing in..." : "Sign in"}
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
