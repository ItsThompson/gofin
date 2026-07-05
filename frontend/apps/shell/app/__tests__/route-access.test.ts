import { describe, it, expect } from "vitest";
import { buildUser } from "@gofin/test-utils";
import { canAccess } from "@/lib/route-access";

// canAccess is the single guard predicate the auth layout applies to a route's
// handle.access. This truth table pins its contract across
// {public, authenticated, personal, admin} x {direct admin, regular user,
// assumed session}. An assumed session carries role=user with isAssuming=true.
const regularUser = buildUser({ role: "user" });
const adminUser = buildUser({ role: "admin" });

describe("canAccess", () => {
  describe("public and authenticated are role-agnostic", () => {
    it.each(["public", "authenticated"] as const)(
      "%s is reachable by a regular user, a direct admin, and an assumed session",
      (access) => {
        expect(canAccess(regularUser, false, access)).toBe(true);
        expect(canAccess(adminUser, false, access)).toBe(true);
        expect(canAccess(regularUser, true, access)).toBe(true);
      },
    );
  });

  describe("personal requires a finance-capable (role=user) identity", () => {
    it("allows a regular user", () => {
      expect(canAccess(regularUser, false, "personal")).toBe(true);
    });

    it("allows an assumed session (role=user, isAssuming=true)", () => {
      expect(canAccess(regularUser, true, "personal")).toBe(true);
    });

    it("denies a direct admin", () => {
      expect(canAccess(adminUser, false, "personal")).toBe(false);
    });
  });

  describe("admin requires a direct operator (role=admin, not assuming)", () => {
    it("allows a direct admin", () => {
      expect(canAccess(adminUser, false, "admin")).toBe(true);
    });

    it("denies a regular user", () => {
      expect(canAccess(regularUser, false, "admin")).toBe(false);
    });

    it("denies an admin while assuming a user", () => {
      expect(canAccess(adminUser, true, "admin")).toBe(false);
    });

    it("denies an assumed session (role=user)", () => {
      expect(canAccess(regularUser, true, "admin")).toBe(false);
    });
  });
});
