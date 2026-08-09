import { Button } from "@gofin/ui/components/button";
import { ServerCrash } from "lucide-react";
import type { BackendUnavailableProps } from "./types";

/**
 * Rendered when the auth check could not complete, so whether a session exists
 * is unknown. Distinct from the /login redirect: nothing established that the
 * session ended, so the user is not told it did.
 */
export function BackendUnavailable({ onRetry }: BackendUnavailableProps) {
  return (
    <div
      role="alert"
      className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 text-center"
    >
      <ServerCrash className="size-12 text-muted-foreground" />
      <div>
        <h1 className="text-2xl font-bold">GoFin is unreachable</h1>
        <p className="mt-1 text-muted-foreground">
          We could not reach the server to check your session. You have not been
          signed out.
        </p>
      </div>
      <Button onClick={onRetry}>Try again</Button>
    </div>
  );
}
