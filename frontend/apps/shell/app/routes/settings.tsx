import { lazy, Suspense } from "react";

/**
 * Lazy-load the SettingsPage from the finance remote package.
 */
const SettingsPage = lazy(() =>
  import("@gofin/finance/src/pages/SettingsPage").then((mod) => ({
    default: mod.SettingsPage,
  })),
);

export default function SettingsRoute() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-muted-foreground">Loading settings...</div>
        </div>
      }
    >
      <SettingsPage />
    </Suspense>
  );
}
