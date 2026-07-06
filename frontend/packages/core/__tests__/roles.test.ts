import { describe, it, expect } from "vitest";
import type { User } from "../src/types";
import {
  canUseFinanceFeatures,
  canUseAdminFeatures,
  getLandingPath,
} from "../src/roles";

// Local fixture: core is a dependency-free leaf package, so it cannot import
// the shared `buildUser` factory from @gofin/test-utils (which depends on
// @gofin/core) without creating a circular dependency. Role is the only field
// under test; the rest are valid, representative values.
function makeUser(role: User["role"]): User {
  return {
    id: "user-1",
    username: "testuser",
    email: "test@example.com",
    role,
    currency: "USD",
    hasCompletedOnboarding: true,
    createdAt: "2026-01-01T00:00:00.000Z",
  };
}

const regularUser = makeUser("user");
const adminUser = makeUser("admin");

describe("canUseFinanceFeatures", () => {
  it("is true for a regular user (role=user)", () => {
    expect(canUseFinanceFeatures(regularUser)).toBe(true);
  });

  it("is false for an admin user (role=admin)", () => {
    expect(canUseFinanceFeatures(adminUser)).toBe(false);
  });
});

describe("canUseAdminFeatures", () => {
  it("is true for an admin user (role=admin)", () => {
    expect(canUseAdminFeatures(adminUser)).toBe(true);
  });

  it("is false for a regular user (role=user)", () => {
    expect(canUseAdminFeatures(regularUser)).toBe(false);
  });
});

describe("getLandingPath", () => {
  it("returns /admin for an admin user", () => {
    expect(getLandingPath(adminUser)).toBe("/admin");
  });

  it("returns /dashboard for a regular user", () => {
    expect(getLandingPath(regularUser)).toBe("/dashboard");
  });
});
