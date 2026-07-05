import {
  LayoutDashboard,
  Receipt,
  History,
  Settings,
  Shield,
} from "lucide-react";
import type { NavLinkItem } from "./types";

/**
 * navLinksFor returns the navbar entries for the current identity. A direct
 * admin (operator, not assuming) gets the operator surface; a regular user or
 * an assumed session gets the finance surface. Pure and testable: the caller
 * decides who is a direct admin.
 */
export function navLinksFor(isDirectAdmin: boolean): NavLinkItem[] {
  if (isDirectAdmin) {
    return [
      { to: "/admin", label: "Admin", icon: Shield },
      { to: "/settings", label: "Settings", icon: Settings },
    ];
  }
  return [
    { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
    { to: "/expenses", label: "Expenses", icon: Receipt },
    { to: "/history", label: "History", icon: History },
    { to: "/settings", label: "Settings", icon: Settings },
  ];
}
