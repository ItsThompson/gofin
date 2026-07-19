import { Link } from "react-router";
import { Button } from "@gofin/ui/components/button";
import type { LandingHeaderProps } from "../types";

/**
 * Sticky marketing header: brand wordmark (links home) and a real "Log in"
 * link. Reuses the app navbar's sticky/blur treatment for visual consistency.
 */
export function LandingHeader({ brand, login }: LandingHeaderProps) {
  return (
    <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center px-4">
        <Link to="/" className="flex items-center gap-2">
          <span className="text-lg font-bold">{brand}</span>
        </Link>
        <div className="flex-1" />
        <Button asChild variant="ghost">
          <Link to={login.href}>{login.label}</Link>
        </Button>
      </div>
    </header>
  );
}
