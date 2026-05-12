import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import { UserActionsCell } from "./UserActionsCell";
import type { AdminUser } from "../../types";
import type { UserDeletionState } from "../../hooks/useUserDeletion";
import type { User } from "@gofin/core";

interface UsersTableProps {
  users: AdminUser[];
  currentUser: User | null;
  assumingUserId: string | null;
  deletionStates: UserDeletionState["deletionStates"];
  onAssume: (userId: string) => void;
  onDelete: (user: { id: string; username: string }) => void;
}

export function UsersTable({
  users,
  currentUser,
  assumingUserId,
  deletionStates,
  onAssume,
  onDelete,
}: UsersTableProps) {
  return (
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
                    className={`border-b last:border-0 ${isCurrentAdmin ? "bg-primary/5" : ""}`}
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
                          deletionState={deletionStates[user.id]}
                          assumingUserId={assumingUserId}
                          onAssume={onAssume}
                          onDelete={(targetUser) => onDelete({ id: targetUser.id, username: targetUser.username })}
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
  );
}
