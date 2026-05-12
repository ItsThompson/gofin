import { useState, useEffect, useCallback } from "react";
import { Button } from "@gofin/ui/components/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import { apiClient } from "@gofin/api";
import { useApiToast } from "@gofin/api";
import { Shield, Loader2, Activity, ExternalLink } from "lucide-react";
import { DeleteUserDialog } from "../components/DeleteUserDialog";
import { useUserDeletion } from "../hooks/useUserDeletion";
import { UserActionsCell } from "./components/UserActionsCell";
import type { AdminUser, AdminUsersResponse, AdminPanelPageProps } from "../types";

type LoadState = "loading" | "error" | "success";

/**
 * Admin panel page displaying all registered users with identity assumption controls.
 * Exported via Module Federation for the shell to load dynamically.
 */
export function AdminPanelPage({ currentUser, onAssumeIdentity, grafanaUrl = "http://localhost:3002" }: AdminPanelPageProps) {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [assumingUserId, setAssumingUserId] = useState<string | null>(null);
  const { call: toastCall } = useApiToast();

  const handleUserRemoved = useCallback((userId: string) => {
    setUsers((prev) => prev.filter((user) => user.id !== userId));
  }, []);

  const { state: deletionState, actions: deletionActions } = useUserDeletion({
    onUserRemoved: handleUserRemoved,
  });

  const fetchUsers = useCallback(async () => {
    setLoadState("loading");
    const result = await toastCall(async () => {
      const response = await apiClient<AdminUsersResponse>("/api/admin/users");
      return response;
    });
    if (result) {
      setUsers(result.users);
      setLoadState("success");
    } else {
      setLoadState("error");
    }
  }, [toastCall]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleAssume = async (userId: string) => {
    setAssumingUserId(userId);
    try {
      await onAssumeIdentity(userId);
    } catch {
      // Toast handles the error notification
      setAssumingUserId(null);
    }
  };

  if (loadState === "loading") {
    return (
      <div className="flex min-h-[300px] items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
        <span className="ml-2 text-muted-foreground">Loading users...</span>
      </div>
    );
  }

  if (loadState === "error") {
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
          <Button variant="outline" onClick={fetchUsers}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Shield className="size-6 text-primary" />
        <div>
          <h1 className="text-2xl font-bold">Admin Panel</h1>
          <p className="text-sm text-muted-foreground">
            Manage users and assume identities for support
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>System Monitoring</CardTitle>
          <CardDescription>
            View system health, service metrics, and performance dashboards
          </CardDescription>
        </CardHeader>
        <CardContent>
          <a
            href={grafanaUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            <Activity className="size-4" />
            Open Grafana Dashboards
            <ExternalLink className="size-3" />
          </a>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Registered Users</CardTitle>
          <CardDescription>
            {users.length} {users.length === 1 ? "user" : "users"} registered
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left">
                  <th className="pb-2 pr-4 font-medium text-muted-foreground">Username</th>
                  <th className="pb-2 pr-4 font-medium text-muted-foreground">Email</th>
                  <th className="pb-2 pr-4 font-medium text-muted-foreground">Role</th>
                  <th className="pb-2 pr-4 font-medium text-muted-foreground">Registered</th>
                  <th className="pb-2 font-medium text-muted-foreground">Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => {
                  const isCurrentAdmin = user.id === currentUser?.id;

                  return (
                    <tr
                      key={user.id}
                      className={`border-b last:border-0 ${
                        isCurrentAdmin ? "bg-primary/5" : ""
                      }`}
                    >
                      <td className="py-3 pr-4">
                        <span className="font-medium">{user.username}</span>
                        {isCurrentAdmin && (
                          <span className="ml-2 inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                            You
                          </span>
                        )}
                      </td>
                      <td className="py-3 pr-4 text-muted-foreground">{user.email}</td>
                      <td className="py-3 pr-4">
                        <span
                          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                            user.role === "admin"
                              ? "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400"
                              : "bg-muted text-muted-foreground"
                          }`}
                        >
                          {user.role}
                        </span>
                      </td>
                      <td className="py-3 pr-4 text-muted-foreground">
                        {new Date(user.createdAt).toLocaleDateString()}
                      </td>
                      <td className="py-3">
                        {!isCurrentAdmin && (
                          <UserActionsCell
                            user={user}
                            deletionState={deletionState.deletionStates[user.id]}
                            assumingUserId={assumingUserId}
                            onAssume={handleAssume}
                            onDelete={(user) => deletionActions.startDeletion({ id: user.id, username: user.username })}
                          />
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <DeleteUserDialog
        open={deletionState.deletingUser !== null}
        onOpenChange={(open) => {
          if (!open) deletionActions.cancelDeletion();
        }}
        user={deletionState.deletingUser}
        onSuccess={deletionActions.handleDeletionSuccess}
      />
    </div>
  );
}
