import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { SettingsFeature } from "@/features/settings";
import { buildUser } from "@gofin/test-utils";

const regularUser = buildUser({
  role: "user",
  username: "alice",
  email: "alice@example.com",
});

const mockFetch = vi.fn();
global.fetch = mockFetch;

const adminUser = buildUser({
  role: "admin",
  username: "operator",
  email: "operator@example.com",
});

function renderAdminSettings() {
  return render(
    <MemoryRouter>
      <SettingsFeature user={adminUser} onUserUpdated={vi.fn()} />
    </MemoryRouter>,
  );
}

describe("SettingsFeature - admin composition", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("renders exactly the Profile and Password tabs", () => {
    renderAdminSettings();

    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.getAllByText("Profile").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Password").length).toBeGreaterThanOrEqual(1);

    // Finance-only tabs are absent from the admin tab list.
    expect(screen.queryByText("Default Budget")).not.toBeInTheDocument();
    expect(screen.queryByText("Tags")).not.toBeInTheDocument();
  });

  it("defaults to the Profile tab, showing the profile form on mount", () => {
    renderAdminSettings();

    // Profile is tabList[0] for an admin, so its fields render without any
    // click. Both the desktop card and the default-expanded mobile accordion
    // render the section, hence getAllByLabelText.
    const usernameInputs = screen.getAllByLabelText("Username") as HTMLInputElement[];
    expect(usernameInputs.length).toBeGreaterThanOrEqual(1);
    expect(usernameInputs[0].value).toBe("operator");

    const emailInputs = screen.getAllByLabelText("Email") as HTMLInputElement[];
    expect(emailInputs[0].value).toBe("operator@example.com");
  });

  it("does not render the Data Export section on the Profile tab", () => {
    renderAdminSettings();

    expect(screen.queryByText("Data Export")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /export my data/i }),
    ).not.toBeInTheDocument();
  });

  it("does not render Default Budget or Tags sections or their inputs", () => {
    renderAdminSettings();

    expect(screen.queryByLabelText("Monthly Budget")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /save defaults/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("New tag name")).not.toBeInTheDocument();
  });

  it("issues no finance API calls for an admin (sections never mount)", () => {
    renderAdminSettings();

    // DefaultBudget, Tags, and Export sections all fetch on mount. None are on
    // the admin render path, so no request is made.
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("falls back to the first tab when activeTab is not in the current tab list", async () => {
    // A default fetch response keeps the regular-user sections (which fetch on
    // mount) quiet during the initial render.
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ defaults: null, tags: [], data: [], total: 0 }),
    });

    // Start as a regular user: activeTab defaults to "budget" (userTabs[0]).
    const { rerender } = render(
      <MemoryRouter>
        <SettingsFeature user={regularUser} onUserUpdated={vi.fn()} />
      </MemoryRouter>,
    );
    // Let the regular-user sections settle their on-mount fetches before the
    // role flip so no state update lands on an unmounted section.
    await waitFor(() =>
      expect(screen.getAllByLabelText("Monthly Budget").length).toBeGreaterThanOrEqual(1),
    );

    // Role flips to admin mid-session on the SAME component instance (this is
    // what the auth store does via onUserUpdated -> checkAuth: it re-renders
    // with a new user, it does not remount). adminTabs = [profile, password]
    // no longer contains "budget", so the persisted activeTab diverges.
    rerender(
      <MemoryRouter>
        <SettingsFeature user={adminUser} onUserUpdated={vi.fn()} />
      </MemoryRouter>,
    );

    // The fallback resolves activeDefinition to tabList[0] (Profile), so the
    // desktop card renders the profile form instead of an empty card. With the
    // old optional chaining this would render nothing.
    const usernameInputs = screen.getAllByLabelText("Username") as HTMLInputElement[];
    expect(usernameInputs.length).toBeGreaterThanOrEqual(1);
    expect(usernameInputs[0].value).toBe("operator");
  });
});
