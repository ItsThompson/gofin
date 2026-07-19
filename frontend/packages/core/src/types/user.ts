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
