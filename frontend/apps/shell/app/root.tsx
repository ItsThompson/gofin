import { useEffect } from "react";
import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
} from "react-router";

import { reportError } from "@gofin/api";
import { Toaster } from "@gofin/ui/components/sonner";
import type { Route } from "./+types/root";
import "./index.css";

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <Toaster />
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  // Reported from an effect rather than the render body: effects do not run on
  // the server, which makes entry.server.tsx's handleError the sole SSR owner
  // and this boundary the sole client-side one, with no origin test to write.
  //
  // One report per distinct error depends on the effect body running once per
  // mount, and it does only because there is no StrictMode anywhere in
  // apps/shell: reportError is not idempotent, so introducing StrictMode later
  // needs a ref keyed on error identity here.
  useEffect(() => {
    if (isRouteErrorResponse(error) && error.status === 404) return;
    reportError(error, {
      kind: "internal",
      op: "render.route",
      domain: "platform",
    });
  }, [error]);

  let message = "Oops!";
  let details = "An unexpected error occurred.";
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    message = error.status === 404 ? "404" : "Error";
    details =
      error.status === 404
        ? "The requested page could not be found."
        : error.statusText || details;
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-4">
      <h1 className="text-4xl font-bold">{message}</h1>
      <p className="mt-2 text-muted-foreground">{details}</p>
      {stack && (
        <pre className="mt-4 w-full max-w-2xl overflow-x-auto rounded-lg bg-muted p-4 text-sm">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}
