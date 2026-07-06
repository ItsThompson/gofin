import { NavLink } from "react-router";
import type { DesktopNavProps } from "./types";

/** Desktop navigation links, hidden on mobile. */
export function DesktopNav({ navLinks }: DesktopNavProps) {
  return (
    <nav className="hidden items-center gap-1 md:flex">
      {navLinks.map((link) => (
        <NavLink
          key={link.to}
          to={link.to}
          className={({ isActive }) =>
            `flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
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
    </nav>
  );
}
