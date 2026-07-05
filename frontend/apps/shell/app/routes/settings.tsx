import { lazy, useCallback } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { SettingsSkeleton } from "@gofin/ui/components/skeletons";

/**
 * Lazy-load the SettingsFeature from the finance remote package.
 */
const SettingsPage = lazy(() =>
  import("@gofin/finance/src/features/settings").then((mod) => ({
    default: mod.SettingsFeature,
  })),
);

export const handle = { access: "authenticated" as const };

export default function SettingsRoute() {
  const { user, checkAuth } = useAuthStore();

  const handleUserUpdated = useCallback(() => {
    checkAuth();
  }, [checkAuth]);

  if (!user) return null;

  return (
    <RemoteBoundary
      sectionName="Settings"
      loadingFallback={<SettingsSkeleton />}
    >
      <SettingsPage user={user} onUserUpdated={handleUserUpdated} />
    </RemoteBoundary>
  );
}
