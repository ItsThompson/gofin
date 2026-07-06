import { NavLink } from "react-router";
import { Button } from "@gofin/ui/components/button";
import { ShieldAlert } from "lucide-react";

/**
 * 403 page rendered inside the layout chrome when the matched route's access
 * level rejects the current identity (e.g. a direct admin opening a personal
 * route by URL, or a regular user opening an admin route). Keeping the navbar
 * means the operator can navigate away instead of being silently redirected.
 */
export function Forbidden() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-24 text-center">
      <ShieldAlert className="size-12 text-muted-foreground" />
      <div>
        <h1 className="text-2xl font-bold">403: Access denied</h1>
        <p className="mt-1 text-muted-foreground">
          You don&apos;t have access to this page.
        </p>
      </div>
      <NavLink to="/">
        <Button variant="default">Go back</Button>
      </NavLink>
    </div>
  );
}
