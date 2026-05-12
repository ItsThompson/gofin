import { Button } from "@gofin/ui/components/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import { Loader2 } from "lucide-react";
import { DeleteUserDialog } from "../components/DeleteUserDialog";
import { useAdminPanel } from "../hooks/useAdminPanel";
import { PageHeader } from "./components/PageHeader";
import { GrafanaCard } from "./components/GrafanaCard";
import { UsersTable } from "./components/UsersTable";
import type { AdminPanelPageProps } from "../types";

/**
 * Admin panel page displaying all registered users with identity assumption controls.
 * Exported via Module Federation for the shell to load dynamically.
 */
export function AdminPanelPage({ currentUser, onAssumeIdentity, grafanaUrl = "http://localhost:3002" }: AdminPanelPageProps) {
  const { state, actions } = useAdminPanel({ currentUser, onAssumeIdentity });

  if (state.loadState === "loading") {
    return (
      <div className="flex min-h-[300px] items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
        <span className="ml-2 text-muted-foreground">Loading users...</span>
      </div>
    );
  }

  if (state.loadState === "error") {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-destructive">Something went wrong</CardTitle>
          <CardDescription>
            Could not load the admin panel. The error details were shown in a
            notification.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" onClick={actions.retry}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader />
      <GrafanaCard grafanaUrl={grafanaUrl} />
      <UsersTable
        users={state.users}
        currentUser={currentUser}
        assumingUserId={state.assumingUserId}
        deletionStates={state.deletion.deletionStates}
        onAssume={actions.handleAssume}
        onDelete={actions.deletion.startDeletion}
      />
      <DeleteUserDialog
        open={state.deletion.deletingUser !== null}
        onOpenChange={(open) => {
          if (!open) actions.deletion.cancelDeletion();
        }}
        user={state.deletion.deletingUser}
        onSuccess={actions.deletion.handleDeletionSuccess}
      />
    </div>
  );
}
