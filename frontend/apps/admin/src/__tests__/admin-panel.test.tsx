import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AdminPanelPage } from "@/pages/AdminPanelPage";
import type { User } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockAdmin: User = {
  id: "admin-1",
  username: "admin",
  email: "admin@gofin.local",
  role: "admin",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockThompsonAdmin: User = {
  id: "thompson-1",
  username: "thompson",
  email: "thompson@gofin.local",
  role: "admin",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockUsers = [
  { id: "user-1", username: "alice", email: "alice@example.com", role: "user", createdAt: "2026-01-15T00:00:00Z" },
  { id: "admin-1", username: "admin", email: "admin@gofin.local", role: "admin", createdAt: "2026-01-01T00:00:00Z" },
  { id: "user-2", username: "bob", email: "bob@example.com", role: "user", createdAt: "2026-02-01T00:00:00Z" },
  { id: "thompson-1", username: "thompson", email: "thompson@gofin.local", role: "admin", createdAt: "2026-01-01T00:00:00Z" },
];

const mockDeletionJob = {
  id: "job-1",
  userId: "user-1",
  status: "pending",
  error: null,
  createdAt: "2026-05-10T00:00:00Z",
  completedAt: null,
};

function mockFetchSuccess() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ users: mockUsers }),
  });
}

function mockFetchError(message: string) {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 500,
    json: () => Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message }),
  });
}

function mockDeletionPostSuccess(job = mockDeletionJob) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 202,
    json: () => Promise.resolve(job),
  });
}

function mockDeletionPollResponse(status: string, error: string | null = null) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({
      ...mockDeletionJob,
      status,
      error,
      completedAt: status === "completed" ? "2026-05-10T00:01:00Z" : null,
    }),
  });
}

describe("AdminPanelPage", () => {
  const mockOnAssume = vi.fn();

  beforeEach(() => {
    mockFetch.mockReset();
    mockOnAssume.mockReset();
  });

  it("renders loading state initially", () => {
    // Never resolve the fetch
    mockFetch.mockReturnValueOnce(new Promise(() => {}));
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    expect(screen.getByText("Loading users...")).toBeInTheDocument();
  });

  it("renders user table on successful fetch", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    expect(screen.getByText("bob")).toBeInTheDocument();
    expect(screen.getAllByText("admin").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("4 users registered")).toBeInTheDocument();
  });

  it("highlights current admin with 'You' badge", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("You")).toBeInTheDocument();
    });
  });

  it("shows Assume button for non-admin users only", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    // There should be Assume buttons for alice, bob, and thompson, but not for admin (current user)
    const assumeButtons = screen.getAllByRole("button", { name: /assume/i });
    expect(assumeButtons).toHaveLength(3);
  });

  it("calls onAssumeIdentity when Assume is clicked", async () => {
    mockFetchSuccess();
    mockOnAssume.mockResolvedValueOnce(undefined);

    const user = userEvent.setup();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    const assumeButtons = screen.getAllByRole("button", { name: /assume/i });
    await user.click(assumeButtons[0]);

    expect(mockOnAssume).toHaveBeenCalledWith("user-1");
  });

  it("renders error state on fetch failure", async () => {
    mockFetchError("Server error");
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("renders System Monitoring section with Grafana link", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("System Monitoring")).toBeInTheDocument();
    });

    const grafanaLink = screen.getByRole("link", { name: /open grafana dashboards/i });
    expect(grafanaLink).toHaveAttribute("href", "http://localhost:3002");
    expect(grafanaLink).toHaveAttribute("target", "_blank");
    expect(grafanaLink).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("uses custom Grafana URL when provided", async () => {
    mockFetchSuccess();
    render(
      <AdminPanelPage
        currentUser={mockAdmin}
        onAssumeIdentity={mockOnAssume}
        grafanaUrl="https://grafana.example.com"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("System Monitoring")).toBeInTheDocument();
    });

    const grafanaLink = screen.getByRole("link", { name: /open grafana dashboards/i });
    expect(grafanaLink).toHaveAttribute("href", "https://grafana.example.com");
  });

  it("retries fetch on retry button click", async () => {
    mockFetchError("Server error");
    const user = userEvent.setup();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    });

    mockFetchSuccess();
    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });
  });

  // --- Delete Button Visibility Tests ---

  it("shows delete button for non-protected, non-self users only", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    // Delete buttons should appear for alice and bob only
    const deleteButtons = screen.getAllByRole("button", { name: /delete/i });
    expect(deleteButtons).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Delete alice" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete bob" })).toBeInTheDocument();
  });

  it("hides delete button for protected user thompson", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("thompson")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: "Delete thompson" })).not.toBeInTheDocument();
  });

  it("hides delete button for current admin (self)", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: "Delete admin" })).not.toBeInTheDocument();
  });

  it("hides delete button when logged in as thompson viewing admin", async () => {
    mockFetchSuccess();
    render(<AdminPanelPage currentUser={mockThompsonAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: "Delete admin" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete thompson" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete alice" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete bob" })).toBeInTheDocument();
  });

  it("opens delete dialog when trashcan button is clicked", async () => {
    mockFetchSuccess();
    const user = userEvent.setup();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete alice" }));

    expect(screen.getByText(/Delete User: alice/)).toBeInTheDocument();
  });

  // --- Deletion State Tests ---

  it("shows spinner and 'Deleting...' for pending/running user", async () => {
    mockFetchSuccess();
    const user = userEvent.setup();
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete alice" }));
    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    mockDeletionPostSuccess();
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Deleting...")).toBeInTheDocument();
    });

    // Assume and Delete buttons should NOT be present for alice
    expect(screen.queryByRole("button", { name: "Delete alice" })).not.toBeInTheDocument();
  });

  it("removes user from table after polling returns completed", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockFetchSuccess();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const { toast } = await import("sonner");
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete alice" }));
    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    mockDeletionPostSuccess();
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Deleting...")).toBeInTheDocument();
    });

    mockDeletionPollResponse("completed");
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.queryByText("alice")).not.toBeInTheDocument();
    });

    expect(screen.getByText("bob")).toBeInTheDocument();
    expect(toast.success).toHaveBeenCalledWith('User "alice" has been deleted');
    vi.useRealTimers();
  });

  it("shows Failed badge and re-enables Delete button after polling returns failed", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockFetchSuccess();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const { toast } = await import("sonner");
    render(<AdminPanelPage currentUser={mockAdmin} onAssumeIdentity={mockOnAssume} />);

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete alice" }));
    await user.type(screen.getByLabelText("Confirmation"), "permanently delete");
    await user.click(screen.getByRole("button", { name: "Next" }));

    mockDeletionPostSuccess();
    await user.type(screen.getByLabelText("Your Password"), "mypassword");
    await user.click(screen.getByRole("button", { name: "Delete User" }));

    await waitFor(() => {
      expect(screen.getByText("Deleting...")).toBeInTheDocument();
    });

    mockDeletionPollResponse("failed", "Provider finance failed: connection timeout");
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByText("Failed")).toBeInTheDocument();
    });

    // Delete button should be re-enabled for retry
    expect(screen.getByRole("button", { name: "Delete alice" })).toBeInTheDocument();
    expect(toast.error).toHaveBeenCalledWith(
      'Deletion of "alice" failed: Provider finance failed: connection timeout',
    );
    vi.useRealTimers();
  });
});
