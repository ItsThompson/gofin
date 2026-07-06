import { describe, it, expect } from "vitest";
import { buildUser } from "@gofin/test-utils";
import { getSettingsTabs, adminTabs, userTabs } from "@/features/settings/settingsTabs";

describe("settingsTabs", () => {
  it("exposes admin tabs as exactly [Profile, Password]", () => {
    expect(adminTabs.map((tab) => tab.id)).toEqual(["profile", "password"]);
    expect(adminTabs.map((tab) => tab.label)).toEqual(["Profile", "Password"]);
  });

  it("exposes user tabs as [Default Budget, Profile, Password, Tags]", () => {
    expect(userTabs.map((tab) => tab.id)).toEqual([
      "budget",
      "profile",
      "password",
      "tags",
    ]);
  });

  it("selects the admin subset when the user cannot use finance features", () => {
    const tabs = getSettingsTabs(buildUser({ role: "admin" }));
    expect(tabs).toBe(adminTabs);
    expect(tabs.map((tab) => tab.id)).toEqual(["profile", "password"]);
  });

  it("selects the full user set for a regular user", () => {
    const tabs = getSettingsTabs(buildUser({ role: "user" }));
    expect(tabs).toBe(userTabs);
    expect(tabs.map((tab) => tab.id)).toEqual([
      "budget",
      "profile",
      "password",
      "tags",
    ]);
  });

  it("defaults to Profile for admins and Default Budget for users", () => {
    expect(getSettingsTabs(buildUser({ role: "admin" }))[0].id).toBe("profile");
    expect(getSettingsTabs(buildUser({ role: "user" }))[0].id).toBe("budget");
  });
});
