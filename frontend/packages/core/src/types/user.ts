/** Role assigned to a user account. */
export type UserRole = "user" | "admin";

/** Core user model returned by the auth API. */
export interface User {
  id: string;
  username: string;
  email: string;
  role: UserRole;
  currency: string;
  hasCompletedOnboarding: boolean;
  createdAt: string;
}

/** Subset of {@link User} shown in the admin user list. */
export type AdminUserSummary = Pick<
  User,
  "id" | "username" | "email" | "role" | "createdAt"
>;

/** Response shape from GET /api/admin/users. */
export interface AdminUsersResponse {
  users: AdminUserSummary[];
}
