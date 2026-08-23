import { lazy } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { Skeleton } from "@gofin/ui/components/skeleton";
import { accessHandle } from "@/lib/route-access";

/**
 * Lazy-load the HistoryFeature from the finance package.
 */
const HistoryFeature = lazy(() =>
  import("@gofin/finance/src/features/history").then((mod) => ({
    default: mod.HistoryFeature,
  })),
);

function HistoryLoadingSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Skeleton className="size-6 rounded" />
        <Skeleton className="h-8 w-40" />
      </div>
      <div className="space-y-3">
        <Skeleton className="h-20 w-full rounded-lg" />
        <Skeleton className="h-20 w-full rounded-lg" />
        <Skeleton className="h-20 w-full rounded-lg" />
      </div>
    </div>
  );
}

export const handle = accessHandle("personal");

export default function HistoryRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <RemoteBoundary
      sectionName="History"
      loadingFallback={<HistoryLoadingSkeleton />}
    >
      <HistoryFeature />
    </RemoteBoundary>
  );
}
