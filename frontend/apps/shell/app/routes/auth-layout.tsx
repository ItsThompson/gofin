import { Outlet, NavLink, useNavigate, useLocation } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { canUseAdminFeatures, getLandingPath } from "@gofin/core";
import { useEffect, useState } from "react";
import { Button } from "@gofin/ui/components/button";
import {
  LayoutDashboard,
  Receipt,
  History,
  Settings,
  LogOut,
  Menu,
  X,
  Shield,
  ArrowLeftToLine,
  PlusCircle,
} from "lucide-react";

/**
 * Personal-finance routes an operator (direct admin) must never land on. The
 * direct-admin guard bounces these to /admin; the gateway 403 is the real
 * enforcement, this guard is defense-in-depth to avoid a flash of finance UI.
 */
const FINANCE_ROUTES = [
  "/dashboard",
  "/expenses",
  "/expenses/new",
  "/history",
  "/onboarding",
];

export default function AuthLayout() {
  const {
    user,
    isAuthenticated,
    isAssuming,
    isLoading,
    checkAuth,
    logout,
    restoreIdentity,
  } = useAuthStore();
  const navigate = useNavigate();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      navigate("/login");
    }
  }, [isLoading, isAuthenticated, navigate]);

  // Redirect guards (precedence: direct-admin guard first, then onboarding).
  const location = useLocation();
  const isOnOnboarding = location.pathname === "/onboarding";

  useEffect(() => {
    if (isLoading || !isAuthenticated || !user) return;

    const path = location.pathname;

    // 1. Direct-admin guard (takes precedence): an operator is never routed to
    //    a finance route or /onboarding. Skipped while assuming a user.
    if (
      canUseAdminFeatures(user) &&
      !isAssuming &&
      FINANCE_ROUTES.includes(path)
    ) {
      navigate("/admin");
      return;
    }

    // 2. Onboarding guard (unchanged for users).
    if (!user.hasCompletedOnboarding && path !== "/onboarding") {
      navigate("/onboarding");
    } else if (user.hasCompletedOnboarding && path === "/onboarding") {
      navigate(getLandingPath(user));
    }
  }, [isLoading, isAuthenticated, user, isAssuming, location.pathname, navigate]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return null;
  }

  // Render onboarding page without the nav chrome
  if (!user.hasCompletedOnboarding && isOnOnboarding) {
    return <Outlet />;
  }

  const handleLogout = async () => {
    await logout();
    navigate("/login");
  };

  const handleReturnToAdmin = async () => {
    setIsRestoring(true);
    try {
      await restoreIdentity();
      navigate("/admin");
    } catch {
      setIsRestoring(false);
    }
  };

  // A direct admin (operator, not assuming) gets an operator-only navbar and no
  // Log Expense FAB. A regular user or an assumed session keeps the finance nav.
  const isDirectAdmin = canUseAdminFeatures(user) && !isAssuming;

  const navLinks = isDirectAdmin
    ? [
        { to: "/admin", label: "Admin", icon: Shield },
        { to: "/settings", label: "Settings", icon: Settings },
      ]
    : [
        { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
        { to: "/expenses", label: "Expenses", icon: Receipt },
        { to: "/history", label: "History", icon: History },
        { to: "/settings", label: "Settings", icon: Settings },
      ];

  // FAB is hidden for a direct admin and, as today, while assuming (the Return
  // to Admin control occupies the same corner) and on the new-expense page.
  const showLogExpenseFab =
    !isDirectAdmin && !isAssuming && location.pathname !== "/expenses/new";

  return (
    <div className="min-h-screen bg-background">
      {/* Navbar */}
      <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-14 max-w-7xl items-center px-4">
          {/* Logo */}
          <NavLink
            to={getLandingPath(user)}
            className="mr-6 flex items-center gap-2"
          >
            <span className="text-lg font-bold">GoFin</span>
          </NavLink>

          {/* Desktop nav links */}
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

          {/* Spacer */}
          <div className="flex-1" />

          {/* User menu (desktop) */}
          <div className="hidden items-center gap-3 md:flex">
            <span className="text-sm text-muted-foreground">
              {user.username}
            </span>
            <Button variant="ghost" size="sm" onClick={handleLogout}>
              <LogOut className="size-4" />
              Logout
            </Button>
          </div>

          {/* Mobile hamburger */}
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

        {/* Mobile menu */}
        {mobileMenuOpen && (
          <nav className="border-t px-4 pb-4 pt-2 md:hidden">
            <div className="flex flex-col gap-1">
              {navLinks.map((link) => (
                <NavLink
                  key={link.to}
                  to={link.to}
                  onClick={() => setMobileMenuOpen(false)}
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
                  onClick={handleLogout}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                >
                  <LogOut className="size-4" />
                  Logout
                </button>
              </div>
            </div>
          </nav>
        )}
      </header>

      {/* Floating "Return to Admin" button during identity assumption */}
      {isAssuming && (
        <div className="fixed bottom-6 right-6 z-50">
          <Button
            variant="default"
            size="lg"
            onClick={handleReturnToAdmin}
            disabled={isRestoring}
            className="shadow-lg"
          >
            <ArrowLeftToLine className="size-4" />
            Return to Admin
          </Button>
        </div>
      )}

      {/* Mobile floating "Log Expense" FAB: visible on mobile, hidden when
          already on the new-expense page or when the admin return button is
          visible (to avoid overlap). */}
      {showLogExpenseFab && (
        <div className="fixed bottom-6 right-6 z-40 md:hidden">
          <NavLink to="/expenses/new">
            <Button
              size="lg"
              className="rounded-full shadow-lg size-14"
              aria-label="Log Expense"
            >
              <PlusCircle className="size-6" />
            </Button>
          </NavLink>
        </div>
      )}

      {/* Page content */}
      <main className="mx-auto max-w-7xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
