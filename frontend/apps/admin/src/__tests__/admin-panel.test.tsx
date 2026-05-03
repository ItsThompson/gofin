import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AdminPanelPage } from "@/pages/AdminPanelPage";
import type { User } from "@gofin/types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockAdmin: User = {
  id: "admin-1",
  username: "admin",
  email: "admin@gofin.local",
  role: "admin",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockUsers = [
  { id: "user-1", username: "alice", email: "alice@example.com", role: "user", createdAt: "2026-01-15T00:00:00Z" },
  { id: "admin-1", username: "admin", email: "admin@gofin.local", role: "admin", createdAt: "2026-01-01T00:00:00Z" },
  { id: "user-2", username: "bob", email: "bob@example.com", role: "user", createdAt: "2026-02-01T00:00:00Z" },
];

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
    // "admin" appears in both the username column and the role badge
    expect(screen.getAllByText("admin").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("3 users registered")).toBeInTheDocument();
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

    // There should be Assume buttons for alice and bob, but not for admin (current user)
    const assumeButtons = screen.getAllByRole("button", { name: /assume/i });
    expect(assumeButtons).toHaveLength(2);
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

    // Second fetch succeeds
    mockFetchSuccess();
    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("alice")).toBeInTheDocument();
    });
  });
});
