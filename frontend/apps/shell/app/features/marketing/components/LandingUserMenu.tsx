import { Link } from "react-router";
import { getLandingPath } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { Avatar, AvatarFallback } from "@gofin/ui/components/avatar";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@gofin/ui/components/dropdown-menu";
import { getAvatarInitial } from "../avatar-initial";
import type { LandingUserMenuProps } from "../types";

/**
 * Logged-in header view: a visible Dashboard link plus an avatar dropdown
 * (username label, Dashboard, Settings, Log out). The Dashboard target is
 * role-aware via getLandingPath, so an admin still reaches /admin. Navigation
 * items are real links; Log out invokes the store action.
 */
export function LandingUserMenu({ user, logout }: LandingUserMenuProps) {
  const dashboardPath = getLandingPath(user);

  return (
    <div className="flex items-center gap-2">
      <Button asChild variant="ghost">
        <Link to={dashboardPath}>Dashboard</Link>
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label="Open account menu"
            className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Avatar>
              <AvatarFallback>{getAvatarInitial(user.username)}</AvatarFallback>
            </Avatar>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuLabel>{user.username}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem asChild>
            <Link to={dashboardPath}>Dashboard</Link>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            <Link to="/settings">Settings</Link>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => void logout()}>
            Log out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
