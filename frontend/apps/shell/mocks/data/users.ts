import type { User } from "@gofin/core";

export const adminUser: User = {
  id: "u-admin-001",
  username: "admin",
  email: "admin@gofin.local",
  role: "admin",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-04-01T00:00:00Z",
};

export const regularUser: User = {
  id: "u-user-001",
  username: "alex",
  email: "alex@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-04-10T00:00:00Z",
};

/** The user that the mock API authenticates as. Change this to test different roles. */
export let currentMockUser: User = adminUser;

export function setCurrentMockUser(user: User): void {
  currentMockUser = user;
}

export const allUsers: User[] = [adminUser, regularUser];
