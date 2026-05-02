import { Settings } from "lucide-react";

/**
 * Placeholder settings page. Form implementation is wired in a later ticket.
 * Exported via Module Federation for the shell to load dynamically.
 */
export function SettingsPage() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Settings className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Settings</h1>
      </div>
      <p className="text-muted-foreground">
        Account settings will appear here. Coming soon.
      </p>
    </div>
  );
}
