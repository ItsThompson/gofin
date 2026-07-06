import { lazy } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";
import { accessHandle } from "@/lib/route-access";

/**
 * Lazy-load the DashboardFeature from the finance remote package.
 * The finance remote exports this via Module Federation: the shell
 * loads it as a workspace package import at build time.
 */
const DashboardFeature = lazy(() =>
  import("@gofin/finance/src/features/dashboard/index").then((mod) => ({
    default: mod.DashboardFeature,
  })),
);

export const handle = accessHandle("personal");

export default function DashboardRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <RemoteBoundary
      sectionName="Dashboard"
      loadingFallback={<DashboardSkeleton />}
    >
      <DashboardFeature user={user} />
    </RemoteBoundary>
  );
}
