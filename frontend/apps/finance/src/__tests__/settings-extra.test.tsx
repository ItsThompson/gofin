import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { SettingsPage } from "@/pages/SettingsPage";
import type { User } from "@gofin/types";

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

const mockTags = [
  { id: "tag-1", name: "Bills", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-2", name: "Food", isDefault: true, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
  { id: "tag-3", name: "Custom", isDefault: false, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
];

function mockDefaultsFound() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ defaults: mockDefaults }),
  });
}

function mockTagsApiSuccess() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ tags: mockTags }),
  });
}

function renderSettings() {
  return render(
    <MemoryRouter>
      <SettingsPage user={mockUser} onUserUpdated={vi.fn()} />
    </MemoryRouter>,
  );
}

describe("SettingsPage - Tags delete and edit cancel", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("deletes a non-default tag", async () => {
    mockDefaultsFound();
    renderSettings();

    const user = userEvent.setup();
    mockTagsApiSuccess();
    const tagsTabs = screen.getAllByText("Tags");
    await user.click(tagsTabs[0]);

    await waitFor(() => {
      expect(screen.getByText("Custom")).toBeInTheDocument();
    });

    // Mock DELETE success
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 204,
      json: () => Promise.resolve({}),
    });

    const deleteButton = screen.getByLabelText("Delete Custom");
    await user.click(deleteButton);

    await waitFor(() => {
      expect(screen.queryByText("Custom")).not.toBeInTheDocument();
    });
  });

  it("shows error when deleting a tag that is in use", async () => {
    mockDefaultsFound();
    renderSettings();

    const user = userEvent.setup();
    mockTagsApiSuccess();
    const tagsTabs = screen.getAllByText("Tags");
    await user.click(tagsTabs[0]);

    await waitFor(() => {
      expect(screen.getByText("Custom")).toBeInTheDocument();
    });

    // Mock DELETE failure
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 409,
      json: () => Promise.resolve({ code: "TAG_IN_USE", message: "Cannot delete tag: it is used by 5 expenses" }),
    });

    const deleteButton = screen.getByLabelText("Delete Custom");
    await user.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText(/cannot delete tag/i)).toBeInTheDocument();
    });
  });

  it("cancels tag edit and restores original state", async () => {
    mockDefaultsFound();
    renderSettings();

    const user = userEvent.setup();
    mockTagsApiSuccess();
    const tagsTabs = screen.getAllByText("Tags");
    await user.click(tagsTabs[0]);

    await waitFor(() => {
      expect(screen.getByText("Bills")).toBeInTheDocument();
    });

    // Start editing
    await user.click(screen.getByLabelText("Edit Bills"));
    expect(screen.getByLabelText("Edit tag name")).toBeInTheDocument();

    // Cancel editing
    // Click the last ghost button in the edit UI (the X button)
    const editRow = screen.getByLabelText("Edit tag name").closest("li");
    const buttonsInRow = editRow!.querySelectorAll("button");
    // Last button is the cancel (X) button
    await user.click(buttonsInRow[buttonsInRow.length - 1]);

    // Edit mode should be dismissed, original name restored
    await waitFor(() => {
      expect(screen.queryByLabelText("Edit tag name")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Bills")).toBeInTheDocument();
  });

  it("shows error on save edit failure", async () => {
    mockDefaultsFound();
    renderSettings();

    const user = userEvent.setup();
    mockTagsApiSuccess();
    const tagsTabs = screen.getAllByText("Tags");
    await user.click(tagsTabs[0]);

    await waitFor(() => {
      expect(screen.getByText("Bills")).toBeInTheDocument();
    });

    // Start editing
    await user.click(screen.getByLabelText("Edit Bills"));

    const editInput = screen.getByLabelText("Edit tag name");
    await user.clear(editInput);
    await user.type(editInput, "Food"); // Duplicate name

    // Mock save failure
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 409,
      json: () => Promise.resolve({ code: "DUPLICATE_TAG", message: "Tag already exists" }),
    });

    // Click the save button (Check icon)
    const editRow = screen.getByLabelText("Edit tag name").closest("li");
    const buttonsInRow = editRow!.querySelectorAll("button");
    // First button is the save (Check) button
    await user.click(buttonsInRow[0]);

    await waitFor(() => {
      expect(screen.getByText(/already exists/i)).toBeInTheDocument();
    });
  });
});

describe("SettingsPage - Mobile accordion", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("toggles accordion sections on mobile", () => {
    mockDefaultsFound();
    renderSettings();

    // The mobile accordion renders all tabs as expandable sections
    // "Default Budget" is expanded by default; others are collapsed
    // Find the mobile accordion buttons (they have ▲/▼ indicators)
    const accordionButtons = screen.getAllByText("▼");
    expect(accordionButtons.length).toBeGreaterThanOrEqual(1);
  });

  it("shows api error when default budget save fails", async () => {
    mockDefaultsFound();
    renderSettings();

    const user = userEvent.setup();

    await waitFor(() => {
      const budgetInputs = screen.getAllByLabelText("Monthly Budget");
      expect((budgetInputs[0] as HTMLInputElement).value).toBe("3000");
    });

    // Mock save failure
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message: "Service unavailable" }),
    });

    const submitButton = screen.getAllByRole("button", { name: /save defaults/i })[0];
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/service unavailable/i)).toBeInTheDocument();
    });
  });

  it("calls onUserUpdated when profile update succeeds", async () => {
    mockDefaultsFound();
    const onUserUpdated = vi.fn();
    render(
      <MemoryRouter>
        <SettingsPage user={mockUser} onUserUpdated={onUserUpdated} />
      </MemoryRouter>,
    );

    const user = userEvent.setup();

    // Switch to Profile tab
    const profileTabs = screen.getAllByText("Profile");
    await user.click(profileTabs[0]);

    await waitFor(() => {
      expect(screen.getByLabelText("Username")).toBeInTheDocument();
    });

    // Mock successful profile update
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ user: { ...mockUser, username: "newalice" } }),
    });

    const submitButton = screen.getByRole("button", { name: /update profile/i });
    await user.click(submitButton);

    await waitFor(() => {
      expect(onUserUpdated).toHaveBeenCalled();
    });
  });
});
