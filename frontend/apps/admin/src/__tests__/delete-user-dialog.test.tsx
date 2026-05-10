import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeleteUserDialog } from "@/components/DeleteUserDialog";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockDeletionJob = {
  id: "job-1",
  userId: "user-1",
  status: "pending",
  error: null,
  createdAt: "2026-05-10T00:00:00Z",
  completedAt: null,
};

describe("DeleteUserDialog", () => {
  const mockOnOpenChange = vi.fn();
  const mockOnSuccess = vi.fn();
  const testUser = { id: "user-1", username: "alice" };

  beforeEach(() => {
    mockFetch.mockReset();
    mockOnOpenChange.mockReset();
    mockOnSuccess.mockReset();
  });

  it("renders nothing when user is null", () => {
    const { container } = render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={null}
        onSuccess={mockOnSuccess}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows step 1 with confirmation phrase input", () => {
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    expect(screen.getByText(/Delete User: alice/)).toBeInTheDocument();
    expect(screen.getByText(/permanently delete/)).toBeInTheDocument();
    expect(screen.getByLabelText("Confirmation")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });

  it("Next button disabled until exact phrase is typed", async () => {
    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    const input = screen.getByLabelText("Confirmation");
    const nextButton = screen.getByRole("button", { name: "Next" });

    await user.type(input, "wrong text");
    expect(nextButton).toBeDisabled();

    await user.clear(input);
    await user.type(input, "permanently delete");
    expect(nextButton).toBeEnabled();
  });

  it("advances to step 2 when Next is clicked after valid phrase", async () => {
    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByLabelText("Your Password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete User" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
  });

  it("Back button in step 2 returns to step 1", async () => {
    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("button", { name: "Back" }));

    expect(screen.getByLabelText("Confirmation")).toBeInTheDocument();
  });

  it("calls POST /api/datarights/deletions and passes job to onSuccess", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: () => Promise.resolve(mockDeletionJob),
    });

    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    // Step 1
    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    // Step 2
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalledWith(mockDeletionJob);
    });

    expect(mockOnOpenChange).toHaveBeenCalledWith(false);
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/datarights/deletions",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ userId: "user-1", password: "mypassword" }),
      }),
    );
  });

  it("shows inline error on wrong password (401)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: "INVALID_CREDENTIALS", message: "Invalid password" }),
    });

    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    // Step 1
    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    // Step 2
    await user.type(screen.getByLabelText("Your Password"), "wrongpassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Invalid password")).toBeInTheDocument();
    });

    // Dialog stays open
    expect(mockOnOpenChange).not.toHaveBeenCalledWith(false);
    expect(mockOnSuccess).not.toHaveBeenCalled();
  });

  it("shows inline error on 403 protected user", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ code: "PROTECTED_USER", message: "Cannot delete a protected user" }),
    });

    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Cannot delete a protected user")).toBeInTheDocument();
    });

    expect(mockOnSuccess).not.toHaveBeenCalled();
  });

  it("shows inline error on 409 export in progress", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 409,
      json: () => Promise.resolve({ code: "EXPORT_CONFLICT", message: "Cannot delete user while data export is in progress" }),
    });

    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Cannot delete user while data export is in progress")).toBeInTheDocument();
    });

    expect(mockOnSuccess).not.toHaveBeenCalled();
  });

  it("shows inline error on 400 self-deletion", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ code: "BAD_REQUEST", message: "Cannot delete your own account" }),
    });

    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Cannot delete your own account")).toBeInTheDocument();
    });

    expect(mockOnSuccess).not.toHaveBeenCalled();
  });

  it("Delete User button is disabled when password is empty", async () => {
    const user = userEvent.setup();
    render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByRole("button", { name: "Delete User" })).toBeDisabled();
  });

  it("resets to step 1 when dialog is closed and reopened", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    // Navigate to step 2
    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByLabelText("Your Password")).toBeInTheDocument();

    // Close
    rerender(
      <DeleteUserDialog
        open={false}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    // Reopen
    rerender(
      <DeleteUserDialog
        open={true}
        onOpenChange={mockOnOpenChange}
        user={testUser}
        onSuccess={mockOnSuccess}
      />,
    );

    // Should be back on step 1
    expect(screen.getByLabelText("Confirmation")).toBeInTheDocument();
  });
});
