import { lazy, Suspense } from "react";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Lazy-load the SettingsPage from the finance remote package.
 */
const SettingsPage = lazy(() =>
  import("@gofin/finance/src/pages/SettingsPage").then((mod) => ({
    default: mod.SettingsPage,
  })),
);

export default function SettingsRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <Suspense
      fallback={
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-muted-foreground">Loading settings...</div>
        </div>
      }
    >
      <SettingsPage user={user} />
    </Suspense>
  );
}
