import { NavLink } from "react-router";
import { LogOut } from "lucide-react";
import type { MobileNavProps } from "./types";

/**
 * Mobile navigation menu, shown when the hamburger is open. Renders the same
 * links as the desktop nav plus the username and a logout action. Clicking a
 * link fires onNavigate so the parent can close the menu.
 */
export function MobileNav({
  navLinks,
  user,
  open,
  onNavigate,
  onLogout,
}: MobileNavProps) {
  if (!open) {
    return null;
  }

  return (
    <nav className="border-t px-4 pb-4 pt-2 md:hidden">
      <div className="flex flex-col gap-1">
        {navLinks.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            onClick={onNavigate}
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                isActive
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              }`
            }
          >
            <link.icon className="size-4" />
            {link.label}
          </NavLink>
        ))}
        <div className="mt-2 border-t pt-2">
          <div className="px-3 py-1 text-sm text-muted-foreground">
            {user.username}
          </div>
          <button
            onClick={onLogout}
            className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          >
            <LogOut className="size-4" />
            Logout
          </button>
        </div>
      </div>
    </nav>
  );
}
