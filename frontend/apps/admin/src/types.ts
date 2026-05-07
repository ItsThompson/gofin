import type { User } from "@gofin/core";

/** Represents a user in the admin user list. */
export interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: "user" | "admin";
  createdAt: string;
}

/** Response shape from GET /api/admin/users. */
export interface AdminUsersResponse {
  users: AdminUser[];
}

/** Props for the AdminPanelPage component. */
export interface AdminPanelPageProps {
  currentUser: User | null;
  onAssumeIdentity: (userId: string) => Promise<void>;
  /** URL of the Grafana auth proxy. Defaults to http://localhost:3002. */
  grafanaUrl?: string;
}
