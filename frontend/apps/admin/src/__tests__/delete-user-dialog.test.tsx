import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeleteUserDialog } from "@/components/DeleteUserDialog";

const mockFetch = vi.fn();
global.fetch = mockFetch;

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

  it("calls API and onSuccess on successful deletion", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 204,
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
      expect(mockOnSuccess).toHaveBeenCalled();
    });

    expect(mockOnOpenChange).toHaveBeenCalledWith(false);
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/admin/users/user-1",
      expect.objectContaining({
        method: "DELETE",
        body: JSON.stringify({ password: "mypassword" }),
      }),
    );
  });

  it("shows inline error on wrong password (401)", async () => {
    // With /api/admin/users in AUTH_ENDPOINT_PREFIXES, 401 throws directly
    // without attempting token refresh
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
