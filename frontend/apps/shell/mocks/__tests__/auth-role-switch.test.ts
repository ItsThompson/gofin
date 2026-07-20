import { describe, it, expect, afterEach } from "vitest";
import type { User } from "@gofin/core";
import { authHandlers } from "../handlers/auth";
import { setCurrentMockUser, adminUser, regularUser } from "../data";
import { resolveMockRequest } from "./drive";

describe("GET /api/auth/me role switching via the live currentMockUser binding", () => {
  afterEach(() => setCurrentMockUser(adminUser));

  async function fetchMe(): Promise<{ user: User }> {
    const res = await resolveMockRequest(
      authHandlers,
      "/api/auth/me",
    );
    return res.json();
  }

  it("reflects setCurrentMockUser at request time, not a snapshot from import", async () => {
    const asAdmin = await fetchMe();
    expect(asAdmin.user.role).toBe("admin");
    expect(asAdmin.user.id).toBe(adminUser.id);

    setCurrentMockUser(regularUser);
    const asRegular = await fetchMe();
    expect(asRegular.user.role).toBe("user");
    expect(asRegular.user.id).toBe(regularUser.id);

    setCurrentMockUser(adminUser);
    const backToAdmin = await fetchMe();
    expect(backToAdmin.user.role).toBe("admin");
  });
});
