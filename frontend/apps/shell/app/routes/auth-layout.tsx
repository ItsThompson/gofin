import { Outlet, Navigate, useNavigate, useLocation } from "react-router";
import { useEffect, useState } from "react";
import {
  classifyApiFailure,
  isNetworkError,
  NETWORK_FAILURE,
  reportError,
} from "@gofin/api";
import { toast } from "sonner";
import { useAuthStore } from "@/stores/auth-store";
import { canUseAdminFeatures } from "@gofin/core";
import {
  Navbar,
  ReturnToAdminButton,
  LogExpenseFab,
  Forbidden,
  BackendUnavailable,
  useAuthLayoutGuards,
} from "@/features/shell-layout";

/**
 * AuthLayout is the thin orchestrator for every authenticated route. It reads
 * auth state, delegates the routing decision to useAuthLayoutGuards (which
 * derives behavior from the matched route's handle.access), and composes the
 * shell chrome. Guard logic and presentation live in features/shell-layout.
 */
export default function AuthLayout() {
  const {
    user,
    isAuthenticated,
    isAssuming,
    isLoading,
    authError,
    checkAuth,
    logout,
    restoreIdentity,
  } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isRestoring, setIsRestoring] = useState(false);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const guard = useAuthLayoutGuards({
    user,
    isAuthenticated,
    isAssuming,
    isLoading,
    authError,
  });

  if (guard.status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    );
  }

  if (guard.status === "unavailable") {
    return <BackendUnavailable onRetry={checkAuth} />;
  }

  if (guard.status === "redirect") {
    return <Navigate to={guard.to} replace />;
  }

  // Onboarding renders without chrome so the flow fills the screen.
  if (guard.status === "onboarding") {
    return <Outlet />;
  }

  // forbidden and ready both render the full chrome; the guard guarantees a
  // user is present past the redirect branch.
  if (!user) {
    return null;
  }

  const isDirectAdmin = canUseAdminFeatures(user) && !isAssuming;

  const handleLogout = async () => {
    await logout();
    navigate("/login");
  };

  const handleReturnToAdmin = async () => {
    setIsRestoring(true);
    try {
      await restoreIdentity();
      navigate("/admin");
    } catch (error) {
      // The admin stays in the assumed identity, so re-enabling the button was
      // the only thing they used to see.
      reportError(error, {
        ...(isNetworkError(error)
          ? NETWORK_FAILURE
          : classifyApiFailure(error)),
        op: "auth.restore_identity",
        domain: "auth",
      });
      toast.error("Could not return to your admin account. Try again.");
      setIsRestoring(false);
    }
  };

  // FAB is for finance users only, hidden while assuming (shares the corner
  // with Return to Admin), on a 403 page, and on the new-expense page itself.
  const showLogExpenseFab =
    guard.status === "ready" &&
    !isDirectAdmin &&
    !isAssuming &&
    location.pathname !== "/expenses/new";

  return (
    <div className="min-h-screen bg-background">
      <Navbar
        user={user}
        isDirectAdmin={isDirectAdmin}
        onLogout={handleLogout}
      />

      {isAssuming && (
        <ReturnToAdminButton
          onReturn={handleReturnToAdmin}
          disabled={isRestoring}
        />
      )}

      {showLogExpenseFab && <LogExpenseFab />}

      <main className="mx-auto max-w-7xl px-4 py-6">
        {guard.status === "forbidden" ? <Forbidden /> : <Outlet />}
      </main>
    </div>
  );
}
