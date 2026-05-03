import { randomUUID } from "node:crypto";

/**
 * Generate a unique test user with UUID-based credentials.
 * Each test gets an isolated user to avoid cross-test pollution.
 */
export function createTestUser() {
  const id = randomUUID().slice(0, 8);
  return {
    username: `test-${id}`,
    email: `test-${id}@e2e.gofin.local`,
    password: "TestPass123!",
  };
}

/**
 * Admin credentials loaded from .env.test.
 * These must match the values used by `just seed-admin`.
 */
export function getAdminCredentials() {
  return {
    email: process.env.ADMIN_EMAIL ?? "admin@gofin.local",
    password: process.env.ADMIN_PASSWORD ?? "Admin1234!",
  };
}
