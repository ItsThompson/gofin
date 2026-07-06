import { useState } from "react";
import { NavLink } from "react-router";
import { getLandingPath } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { LogOut, Menu, X } from "lucide-react";
import { DesktopNav } from "./DesktopNav";
import { MobileNav } from "./MobileNav";
import { navLinksFor } from "./nav-links";
import type { NavbarProps } from "./types";

/**
 * Top navigation bar: logo, desktop links, username + logout, and the mobile
 * hamburger that toggles the MobileNav. The link set is derived from whether
 * the identity is a direct admin; the mobile-open state is navbar-local UI.
 */
export function Navbar({ user, isDirectAdmin, onLogout }: NavbarProps) {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const navLinks = navLinksFor(isDirectAdmin);

  return (
    <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center px-4">
        <NavLink
          to={getLandingPath(user)}
          className="mr-6 flex items-center gap-2"
        >
          <span className="text-lg font-bold">GoFin</span>
        </NavLink>

        <DesktopNav navLinks={navLinks} />

        <div className="flex-1" />

        <div className="hidden items-center gap-3 md:flex">
          <span className="text-sm text-muted-foreground">{user.username}</span>
          <Button variant="ghost" size="sm" onClick={onLogout}>
            <LogOut className="size-4" />
            Logout
          </Button>
        </div>

        <Button
          variant="ghost"
          size="icon"
          className="md:hidden"
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          aria-label={mobileMenuOpen ? "Close menu" : "Open menu"}
        >
          {mobileMenuOpen ? (
            <X className="size-5" />
          ) : (
            <Menu className="size-5" />
          )}
        </Button>
      </div>

      <MobileNav
        navLinks={navLinks}
        user={user}
        open={mobileMenuOpen}
        onNavigate={() => setMobileMenuOpen(false)}
        onLogout={onLogout}
      />
    </header>
  );
}
