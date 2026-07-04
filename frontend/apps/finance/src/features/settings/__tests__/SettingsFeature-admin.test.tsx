import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { SettingsFeature } from "@/features/settings";
import { buildUser } from "@gofin/test-utils";

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
});
