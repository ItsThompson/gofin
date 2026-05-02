import type { User } from "@gofin/types";

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
}
