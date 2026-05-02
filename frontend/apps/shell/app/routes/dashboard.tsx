import { lazy, Suspense } from "react";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Lazy-load the DashboardPage from the finance remote package.
 * The finance remote exports this via Module Federation: the shell
 * loads it as a workspace package import at build time.
 */
const DashboardPage = lazy(() =>
  import("@gofin/finance/src/pages/DashboardPage").then((mod) => ({
    default: mod.DashboardPage,
  })),
);

export default function DashboardRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <Suspense
      fallback={
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-muted-foreground">Loading dashboard...</div>
        </div>
      }
    >
      <DashboardPage user={user} />
    </Suspense>
  );
}
