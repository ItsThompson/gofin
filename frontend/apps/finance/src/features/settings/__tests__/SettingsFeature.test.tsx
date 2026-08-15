import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { SettingsFeature } from "@/features/settings";
import type { User } from "@gofin/core";
import { buildUser, createMockApi, renderWithRouter } from "@gofin/test-utils";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockUser: User = {
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockDefaults = {
  userId: "user-1",
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function mockDefaultsFound() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ defaults: mockDefaults }),
  });
}

function mockDefaultsNotFound() {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 404,
    json: () =>
      Promise.resolve({
        code: "NOT_FOUND",
        message: "Default settings not found",
      }),
  });
}

function mockApiSuccess(data: unknown = { user: mockUser }) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve(data),
  });
}

function mockApiError(status: number, code: string, message: string) {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status,
    json: () => Promise.resolve({ code, message }),
  });
}

function renderSettings(user: User = mockUser) {
  return render(
    <MemoryRouter>
      <SettingsFeature user={user} />
    </MemoryRouter>,
  );
}

describe("SettingsFeature", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("renders settings page with four tabs", async () => {
    mockDefaultsFound();
    renderSettings();

    expect(screen.getByText("Settings")).toBeInTheDocument();

    // Each tab label renders 3 times: desktop sidebar + desktop card title + mobile accordion
    expect(screen.getAllByText("Default Budget").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("Profile").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("Password").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("Tags").length).toBeGreaterThanOrEqual(2);
  });

  describe("Default Budget section", () => {
    it("loads and displays default values", async () => {
      mockDefaultsFound();
      renderSettings();

      await waitFor(() => {
        const budgetInput = screen.getAllByLabelText(
          "Monthly Budget",
        )[0] as HTMLInputElement;
        expect(budgetInput.value).toBe("3000");
      });

      // E/D/S pre-filled
      const essentialsInputs = screen.getAllByLabelText("Essentials %");
      expect((essentialsInputs[0] as HTMLInputElement).value).toBe("50");

      const desiresInputs = screen.getAllByLabelText("Desires %");
      expect((desiresInputs[0] as HTMLInputElement).value).toBe("30");

      const savingsInputs = screen.getAllByLabelText("Savings %");
      expect((savingsInputs[0] as HTMLInputElement).value).toBe("20");

      expect(
        screen.getAllByText(/default currency applies only when you create a new budget period/i)[0],
      ).toBeInTheDocument();
    });

    it("updates budget input precision when default currency changes", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      await waitFor(() => {
        expect(screen.getAllByLabelText("Monthly Budget")[0]).toHaveAttribute("step", "0.01");
      });

      await user.selectOptions(screen.getAllByLabelText("Default Currency")[0], "JPY");

      expect(screen.getAllByLabelText("Monthly Budget")[0]).toHaveAttribute("step", "1");
    });

    it("validates E/D/S split sums to 100%", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      await waitFor(() => {
        expect(
          (screen.getAllByLabelText("Essentials %")[0] as HTMLInputElement)
            .value,
        ).toBe("50");
      });

      // Change savings to 19 (total = 99%)
      const savingsInput = screen.getAllByLabelText("Savings %")[0];
      await user.clear(savingsInput);
      await user.type(savingsInput, "19");

      const submitButton = screen.getAllByRole("button", {
        name: /save defaults/i,
      })[0];
      await user.click(submitButton);

      expect(
        screen.getAllByText(/percentages must sum to 100%/i)[0],
      ).toBeInTheDocument();
    });

    it("saves defaults and shows success message", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      await waitFor(() => {
        expect(
          (screen.getAllByLabelText("Monthly Budget")[0] as HTMLInputElement)
            .value,
        ).toBe("3000");
      });

      // Submit with current values
      // Mock: finance defaults PUT, auth GET (fresh profile), auth PUT (currency sync)
      mockApiSuccess({ defaults: mockDefaults });
      mockApiSuccess({ user: mockUser });
      mockApiSuccess({ user: mockUser });

      const submitButton = screen.getAllByRole("button", {
        name: /save defaults/i,
      })[0];
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/default settings updated successfully/i),
        ).toBeInTheDocument();
      });
    });

    it("shows error when saving defaults fails", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      await waitFor(() => {
        expect(
          (screen.getAllByLabelText("Monthly Budget")[0] as HTMLInputElement)
            .value,
        ).toBe("3000");
      });

      mockApiError(500, "INTERNAL_SERVER_ERROR", "Failed to save defaults");

      const submitButton = screen.getAllByRole("button", {
        name: /save defaults/i,
      })[0];
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/failed to save defaults/i),
        ).toBeInTheDocument();
      });
    });

    it("uses fallback defaults when fetch fails", async () => {
      mockDefaultsNotFound();
      renderSettings();

      await waitFor(() => {
        const essentialsInputs = screen.getAllByLabelText("Essentials %");
        expect((essentialsInputs[0] as HTMLInputElement).value).toBe("50");
      });

      const desiresInputs = screen.getAllByLabelText("Desires %");
      expect((desiresInputs[0] as HTMLInputElement).value).toBe("30");

      const savingsInputs = screen.getAllByLabelText("Savings %");
      expect((savingsInputs[0] as HTMLInputElement).value).toBe("20");
    });

    it("renders with zero/empty defaults when API returns a server error", async () => {
      // Use createMockApi to verify the SettingsFeature handles a 500 gracefully
      const savedFetch = global.fetch;
      global.fetch = createMockApi({
        "/api/finance/defaults": {
          status: 500,
          body: { code: "INTERNAL_SERVER_ERROR", message: "Database connection failed" },
        },
      }) as unknown as typeof fetch;

      const settingsUser = buildUser({ currency: "USD" });
      renderWithRouter(<SettingsFeature user={settingsUser} />, { route: "/settings" });

      // Page should still render with fallback defaults (0 budget, 50/30/20 split)
      await waitFor(() => {
        const budgetInputs = screen.getAllByLabelText("Monthly Budget");
        expect((budgetInputs[0] as HTMLInputElement).value).toBe("0");
      });

      const essentialsInputs = screen.getAllByLabelText("Essentials %");
      expect((essentialsInputs[0] as HTMLInputElement).value).toBe("50");

      const desiresInputs = screen.getAllByLabelText("Desires %");
      expect((desiresInputs[0] as HTMLInputElement).value).toBe("30");

      const savingsInputs = screen.getAllByLabelText("Savings %");
      expect((savingsInputs[0] as HTMLInputElement).value).toBe("20");

      // Restore the original mock so subsequent tests aren't affected
      global.fetch = savedFetch;
    });
  });

  describe("Profile section", () => {
    it("shows profile fields with current values", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      // Click the Profile tab
      const profileTabs = screen.getAllByText("Profile");
      await user.click(profileTabs[0]);

      await waitFor(() => {
        const usernameInput = screen.getByLabelText(
          "Username",
        ) as HTMLInputElement;
        expect(usernameInput.value).toBe("alice");
      });

      const emailInput = screen.getByLabelText("Email") as HTMLInputElement;
      expect(emailInput.value).toBe("alice@example.com");
    });

    it("shows confirmation on successful profile update", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      // Switch to Profile tab
      const profileTabs = screen.getAllByText("Profile");
      await user.click(profileTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Username")).toBeInTheDocument();
      });

      // Mock successful response
      mockApiSuccess({ user: { ...mockUser, username: "alice" } });

      const submitButton = screen.getByRole("button", {
        name: /update profile/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/profile updated successfully/i),
        ).toBeInTheDocument();
      });
    });

    it("shows error on duplicate email", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      const profileTabs = screen.getAllByText("Profile");
      await user.click(profileTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Email")).toBeInTheDocument();
      });

      // Change email
      const emailInput = screen.getByLabelText("Email");
      await user.clear(emailInput);
      await user.type(emailInput, "taken@example.com");

      // Mock 409 duplicate email
      mockApiError(409, "DUPLICATE_EMAIL", "An account with this email already exists");

      const submitButton = screen.getByRole("button", {
        name: /update profile/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/an account with this email already exists/i),
        ).toBeInTheDocument();
      });
    });

    it("shows error on duplicate username", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      const profileTabs = screen.getAllByText("Profile");
      await user.click(profileTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Username")).toBeInTheDocument();
      });

      const usernameInput = screen.getByLabelText("Username");
      await user.clear(usernameInput);
      await user.type(usernameInput, "takenuser");

      mockApiError(409, "DUPLICATE_USERNAME", "This username is already taken");

      const submitButton = screen.getByRole("button", {
        name: /update profile/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/this username is already taken/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Password section", () => {
    it("shows password change form", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      const passwordTabs = screen.getAllByText("Password");
      await user.click(passwordTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Current Password")).toBeInTheDocument();
        expect(screen.getByLabelText("New Password")).toBeInTheDocument();
      });
    });

    it("validates password strength client-side", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      const passwordTabs = screen.getAllByText("Password");
      await user.click(passwordTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Current Password")).toBeInTheDocument();
      });

      await user.type(screen.getByLabelText("Current Password"), "OldPass1");
      await user.type(screen.getByLabelText("New Password"), "weak");

      const submitButton = screen.getByRole("button", {
        name: /change password/i,
      });
      await user.click(submitButton);

      expect(
        screen.getByText(/password must be at least 8 characters/i),
      ).toBeInTheDocument();
    });

    it("shows confirmation on successful password change", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      const passwordTabs = screen.getAllByText("Password");
      await user.click(passwordTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Current Password")).toBeInTheDocument();
      });

      await user.type(screen.getByLabelText("Current Password"), "OldPass123");
      await user.type(screen.getByLabelText("New Password"), "NewPass456");

      mockApiSuccess({ user: mockUser });

      const submitButton = screen.getByRole("button", {
        name: /change password/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/password changed successfully/i),
        ).toBeInTheDocument();
      });
    });

    it("shows error on wrong current password", async () => {
      mockDefaultsFound();
      const user = userEvent.setup();
      renderSettings();

      const passwordTabs = screen.getAllByText("Password");
      await user.click(passwordTabs[0]);

      await waitFor(() => {
        expect(screen.getByLabelText("Current Password")).toBeInTheDocument();
      });

      await user.type(
        screen.getByLabelText("Current Password"),
        "WrongPass1",
      );
      await user.type(screen.getByLabelText("New Password"), "NewPass456");

      // /api/auth/me/password is an auth endpoint: 401 passes through
      // directly without triggering the refresh cycle.
      mockApiError(
        401,
        "INVALID_CREDENTIALS",
        "Current password is incorrect",
      );

      const submitButton = screen.getByRole("button", {
        name: /change password/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/current password is incorrect/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Tags section", () => {
    const mockTags = [
      { id: "tag-1", name: "Bills", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
      { id: "tag-2", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
      { id: "tag-3", name: "Custom", isDefault: false, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
    ];

    function mockTagsApiSuccess() {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ tags: mockTags }),
      });
    }

    async function switchToTagsTab() {
      const user = userEvent.setup();
      // Mock tags fetch BEFORE switching tab (component fetches on mount)
      mockTagsApiSuccess();
      const tagsTabs = screen.getAllByText("Tags");
      await user.click(tagsTabs[0]);
      return user;
    }

    it("shows tag list with default badges", async () => {
      mockDefaultsFound();
      renderSettings();

      await switchToTagsTab();

      await waitFor(() => {
        expect(screen.getByText("Bills")).toBeInTheDocument();
      });

      expect(screen.getByText("Food")).toBeInTheDocument();
      expect(screen.getByText("Custom")).toBeInTheDocument();

      // Default tags show badge
      const defaultBadges = screen.getAllByText("Default");
      expect(defaultBadges.length).toBe(2);
    });

    it("adds a new tag", async () => {
      mockDefaultsFound();
      renderSettings();

      const user = await switchToTagsTab();

      await waitFor(() => {
        expect(screen.getByText("Bills")).toBeInTheDocument();
      });

      // Type and submit new tag
      const input = screen.getByLabelText("New tag name");
      await user.type(input, "Groceries");

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: () => Promise.resolve({
          tag: { id: "tag-4", name: "Groceries", isDefault: false, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
        }),
      });

      const addButton = screen.getByRole("button", { name: /add tag/i });
      await user.click(addButton);

      await waitFor(() => {
        expect(screen.getByText("Groceries")).toBeInTheDocument();
      });
    });

    it("shows error on duplicate tag name", async () => {
      mockDefaultsFound();
      renderSettings();

      const user = await switchToTagsTab();

      await waitFor(() => {
        expect(screen.getByText("Bills")).toBeInTheDocument();
      });

      const input = screen.getByLabelText("New tag name");
      await user.type(input, "Bills");

      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: () => Promise.resolve({ code: "DUPLICATE_TAG", message: "A tag named \"Bills\" already exists" }),
      });

      const addButton = screen.getByRole("button", { name: /add tag/i });
      await user.click(addButton);

      await waitFor(() => {
        expect(screen.getByText(/already exists/i)).toBeInTheDocument();
      });
    });

    it("hides delete button for default tags", async () => {
      mockDefaultsFound();
      renderSettings();

      await switchToTagsTab();

      await waitFor(() => {
        expect(screen.getByText("Bills")).toBeInTheDocument();
      });

      // Custom tag has delete button, default tags do not
      expect(screen.getByLabelText("Delete Custom")).toBeInTheDocument();
      expect(screen.queryByLabelText("Delete Bills")).not.toBeInTheDocument();
      expect(screen.queryByLabelText("Delete Food")).not.toBeInTheDocument();
    });

    it("allows renaming a tag via inline edit", async () => {
      mockDefaultsFound();
      renderSettings();

      const user = await switchToTagsTab();

      await waitFor(() => {
        expect(screen.getByText("Bills")).toBeInTheDocument();
      });

      // Click edit on Bills tag
      const editButton = screen.getByLabelText("Edit Bills");
      await user.click(editButton);

      // Should show edit input with current name
      const editInput = screen.getByLabelText("Edit tag name");
      expect((editInput as HTMLInputElement).value).toBe("Bills");

      await user.clear(editInput);
      await user.type(editInput, "Utilities");

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          tag: { id: "tag-1", name: "Utilities", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
        }),
      });

      // Click save (check button)
      const saveButtons = screen.getAllByRole("button");
      const saveButton = saveButtons.find(
        (btn) => btn.closest("li") && btn.querySelector("svg"),
      );
      if (saveButton) await user.click(saveButton);

      await waitFor(() => {
        expect(screen.getByText("Utilities")).toBeInTheDocument();
      });
    });
  });
});
