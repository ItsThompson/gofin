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
import { isHydratedServerError } from "@/lib/hydrated-server-error";
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
  // Reported from an effect rather than the render body: it fires once per
  // distinct error instead of on every re-render, and it does not run during the
  // server render.
  //
  // That alone does not keep entry.server.tsx's handleError the sole owner of an
  // SSR error. React Router serializes the error into the hydration payload and
  // this boundary re-renders for it in the browser, so without the second guard
  // the effect files a second event for an incident the server already owns.
  //
  // One report per distinct error also depends on the effect body running once
  // per mount, and it does only because there is no StrictMode anywhere in
  // apps/shell: reportError is not idempotent, so introducing StrictMode later
  // needs a ref keyed on error identity here.
  useEffect(() => {
    // Sub-500 is not a defect, and the same line is drawn in entry.server.tsx's
    // handleError, so the two halves stay symmetric. Only 404 is reachable
    // today: a client-side 4xx route error needs a loader or an action, and
    // apps/shell has neither.
    if (isRouteErrorResponse(error) && error.status < 500) return;
    if (isHydratedServerError(error)) return;
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
