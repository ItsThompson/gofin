import type { ReactNode } from "react";
import { Link } from "react-router";
import { Button } from "@gofin/ui/components/button";
import { LandingUserMenu } from "./LandingUserMenu";
import type { LandingHeaderProps } from "../types";

/**
 * Sticky marketing header: brand wordmark (links home) and an auth-aware action
 * slot. Logged out renders a "Log in" ghost link; logged in renders the
 * LandingUserMenu (Dashboard link + avatar dropdown). While auth is loading the
 * slot stays empty so the view never flashes from login to avatar. Reuses the
 * app navbar's sticky/blur treatment for visual consistency.
 */
export function LandingHeader({ brand, login, auth }: LandingHeaderProps) {
  const { isLoading, isAuthenticated, user, logout } = auth;

  let authSlot: ReactNode = null;
  if (!isLoading) {
    authSlot =
      isAuthenticated && user ? (
        <LandingUserMenu user={user} logout={logout} />
      ) : (
        <Button asChild variant="ghost">
          <Link to={login.href}>{login.label}</Link>
        </Button>
      );
  }

  return (
    <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center px-4">
        <Link to="/" className="flex items-center gap-2">
          <span className="text-lg font-bold">{brand}</span>
        </Link>
        <div className="flex-1" />
        {authSlot}
      </div>
    </header>
  );
}
