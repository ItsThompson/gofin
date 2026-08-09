import { RefreshCw, WifiOff } from "lucide-react";

interface RemoteLoadErrorProps {
  /** Name shown to the user, e.g. "Dashboard", "Admin Panel". */
  sectionName?: string;
}

/**
 * Fallback UI rendered when a lazily imported section fails to load.
 * Shows a non-alarming message with a page refresh button.
 */
function RemoteLoadError({ sectionName }: RemoteLoadErrorProps) {
  const label = sectionName ?? "this section";

  return (
    <div
      role="alert"
      className="flex min-h-[300px] flex-col items-center justify-center gap-4 text-center"
    >
      <WifiOff className="size-10 text-muted-foreground/50" />
      <div className="space-y-1">
        <p className="text-base font-medium">Could not load {label}</p>
        <p className="text-sm text-muted-foreground">
          Try refreshing the page. If the problem persists, check your
          connection.
        </p>
      </div>
      <button
        type="button"
        onClick={() => window.location.reload()}
        className="inline-flex items-center gap-1.5 rounded-md border px-4 py-2 text-sm font-medium transition-colors hover:bg-muted"
      >
        <RefreshCw className="size-4" />
        Refresh page
      </button>
    </div>
  );
}

export { RemoteLoadError };
export type { RemoteLoadErrorProps };
