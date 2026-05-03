import { lazy } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";

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
    <RemoteBoundary
      sectionName="Dashboard"
      loadingFallback={<DashboardSkeleton />}
    >
      <DashboardPage user={user} />
    </RemoteBoundary>
  );
}
