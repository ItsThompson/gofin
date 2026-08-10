import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ExportDataSection } from "../components/ExportDataSection";
import type { ExportJob } from "../types";
import { createMockApi, mockSequence } from "@gofin/test-utils";

function buildExportJob(overrides?: Partial<ExportJob>): ExportJob {
  return {
    id: "job-1",
    userId: "user-1",
    status: "completed",
    createdAt: "2026-04-01T10:00:00Z",
    completedAt: "2026-04-01T10:01:30Z",
    fileSizeBytes: 24576,
    error: null,
    ...overrides,
  };
}

const emptyListResponse = {
  body: { data: [], total: 0, page: 1, pageSize: 50, hasMore: false },
};

const ONE_DAY_MS = 24 * 60 * 60 * 1000;

function buildFutureRetryAfter(): string {
  return new Date(Date.now() + ONE_DAY_MS).toISOString();
}

function formatDateOnly(isoDate: string): string {
  return isoDate.slice(0, 10);
}

describe("ExportDataSection", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  describe("rendering", () => {
    it("shows loading state initially", () => {
      global.fetch = createMockApi({
        "/api/datarights/exports": emptyListResponse,
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);
      expect(screen.getByText("Loading export history...")).toBeInTheDocument();
    });

    it("shows empty state when no exports exist", async () => {
      global.fetch = createMockApi({
        "/api/datarights/exports": emptyListResponse,
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByText("No exports yet")).toBeInTheDocument();
      });
      expect(
        screen.getByText(/Click "Export My Data" to download/),
      ).toBeInTheDocument();
    });

    it("renders export button in ready state when no recent exports", async () => {
      const oldJob = buildExportJob({
        createdAt: "2025-01-01T10:00:00Z",
        completedAt: "2025-01-01T10:01:30Z",
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: [oldJob], total: 1, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });
    });

    it("shows button disabled with 'Export in progress...' when active job exists", async () => {
      const activeJob = buildExportJob({
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: [activeJob], total: 1, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        const button = screen.getByRole("button", { name: /export in progress/i });
        expect(button).toBeDisabled();
      });
    });

    it("shows cooldown message when rate limited", async () => {
      const recentJob = buildExportJob({
        createdAt: new Date().toISOString(),
        completedAt: new Date().toISOString(),
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: [recentJob], total: 1, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByText(/next export available/i)).toBeInTheDocument();
      });

      const button = screen.getByRole("button", { name: /export my data/i });
      expect(button).toBeDisabled();
    });

    it("renders section title and description", async () => {
      global.fetch = createMockApi({
        "/api/datarights/exports": emptyListResponse,
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByText("Data Export")).toBeInTheDocument();
      });
      expect(
        screen.getByText(/Export all your personal data as a ZIP/),
      ).toBeInTheDocument();
    });
  });

  describe("history table", () => {
    it("displays job history with status badges and file size", async () => {
      const jobs = [
        buildExportJob({
          id: "job-1",
          status: "completed",
          createdAt: "2026-05-09T14:30:00Z",
          completedAt: "2026-05-09T14:31:15Z",
          fileSizeBytes: 24576,
        }),
        buildExportJob({
          id: "job-2",
          status: "failed",
          createdAt: "2026-04-01T10:00:00Z",
          completedAt: "2026-04-01T10:01:30Z",
          fileSizeBytes: null,
          error: "Email delivery failed",
        }),
      ];

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: jobs, total: 2, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      // Both desktop table and mobile cards render: use getAllByText
      await waitFor(() => {
        expect(screen.getAllByText("Completed").length).toBeGreaterThanOrEqual(1);
      });

      expect(screen.getAllByText("Failed").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("24 KB").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("Email delivery failed").length).toBeGreaterThanOrEqual(1);
    });

    it("shows all four status badges", async () => {
      const jobs = [
        buildExportJob({ id: "j1", status: "pending", completedAt: null, fileSizeBytes: null }),
        buildExportJob({ id: "j2", status: "running", completedAt: null, fileSizeBytes: null }),
        buildExportJob({ id: "j3", status: "completed" }),
        buildExportJob({ id: "j4", status: "failed", error: "Timed out" }),
      ];

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: jobs, total: 4, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });
      expect(screen.getAllByText("Running").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("Completed").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("Failed").length).toBeGreaterThanOrEqual(1);
    });

    it("displays human-readable file sizes", async () => {
      const jobs = [
        buildExportJob({ id: "j1", fileSizeBytes: 1048576, createdAt: "2025-01-01T00:00:00Z" }),
      ];

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: jobs, total: 1, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("1 MB").length).toBeGreaterThanOrEqual(1);
      });
    });

    it("shows dash for null completed date and file size", async () => {
      const pendingJob = buildExportJob({
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: [pendingJob], total: 1, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });

      // Both desktop and mobile render dashes for null values
      const dashes = screen.getAllByText("—");
      expect(dashes.length).toBeGreaterThanOrEqual(2);
    });

    it("shows failed job error reason inline", async () => {
      const failedJob = buildExportJob({
        status: "failed",
        error: "Data collection timed out",
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": {
          body: { data: [failedJob], total: 1, page: 1, pageSize: 50, hasMore: false },
        },
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Data collection timed out").length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  describe("export action", () => {
    it("sends POST and adds new job to list on success", async () => {
      const user = userEvent.setup();
      const newJob = buildExportJob({
        id: "new-job",
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: empty list
          { status: 200, body: { data: [], total: 0, page: 1, pageSize: 50, hasMore: false } },
          // POST: create export
          { status: 202, body: { job: newJob } },
          // Poll: return the pending job
          { status: 200, body: { data: [newJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
          // Additional polls (to prevent "mock sequence exhausted" error)
          { status: 200, body: { data: [newJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      await user.click(screen.getByRole("button", { name: /export my data/i }));

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });
    });

    it("handles 429 rate limit gracefully and shows cooldown", async () => {
      const user = userEvent.setup();
      const retryAfter = buildFutureRetryAfter();

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: empty list
          { status: 200, body: { data: [], total: 0, page: 1, pageSize: 50, hasMore: false } },
          // POST: rate limited
          {
            status: 429,
            body: {
              code: "RATE_LIMITED",
              message: `Export limit reached. You can request another export after ${formatDateOnly(retryAfter)}.`,
            },
          },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      await user.click(screen.getByRole("button", { name: /export my data/i }));

      await waitFor(() => {
        expect(screen.getByText(/next export available/i)).toBeInTheDocument();
      });

      expect(screen.getByRole("button", { name: /export my data/i })).toBeDisabled();
    });

    it("shows loading state on button while creating", async () => {
      const user = userEvent.setup();

      // Use a fetch that delays the POST response
      let resolvePost: ((value: Response) => void) | undefined;
      const mockFetch = vi.fn().mockImplementation((_url: string, init?: RequestInit) => {
        const method = init?.method ?? "GET";
        if (method === "POST") {
          return new Promise<Response>((resolve) => {
            resolvePost = resolve;
          });
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({ data: [], total: 0, page: 1, pageSize: 50, hasMore: false }),
            { status: 200, headers: { "content-type": "application/json" } },
          ),
        );
      });
      global.fetch = mockFetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      await user.click(screen.getByRole("button", { name: /export my data/i }));

      // Button should show loading state
      await waitFor(() => {
        expect(screen.getByRole("button", { name: /exporting/i })).toBeDisabled();
      });

      // Resolve the POST
      if (resolvePost) {
        const newJob = buildExportJob({ id: "new-job", status: "pending", completedAt: null, fileSizeBytes: null });
        await act(async () => {
          resolvePost!(
            new Response(JSON.stringify({ job: newJob }), {
              status: 202,
              headers: { "content-type": "application/json" },
            }),
          );
        });
      }

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  describe("polling", () => {
    it("polls when active jobs exist and stops when all terminal", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });

      const pendingJob = buildExportJob({
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });
      const completedJob = buildExportJob({
        status: "completed",
        fileSizeBytes: 24576,
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: pending job
          { status: 200, body: { data: [pendingJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
          // Poll 1: still pending
          { status: 200, body: { data: [pendingJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
          // Poll 2: completed
          { status: 200, body: { data: [completedJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });

      // Advance timer to trigger first poll (async to flush pending promises)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      // Advance timer to trigger second poll
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await waitFor(() => {
        expect(screen.getAllByText("Completed").length).toBeGreaterThanOrEqual(1);
      });

      vi.useRealTimers();
    });

    it("continues polling on transient network error (resilient)", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });

      const pendingJob = buildExportJob({
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });
      const completedJob = buildExportJob({
        status: "completed",
        fileSizeBytes: 24576,
      });

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: pending job
          { status: 200, body: { data: [pendingJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
          // Poll 1: network error (500)
          { status: 500, body: { code: "INTERNAL_ERROR", message: "Transient failure" } },
          // Poll 2: recovered, completed
          { status: 200, body: { data: [completedJob], total: 1, page: 1, pageSize: 50, hasMore: false } },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });

      // Poll 1: fails transiently
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      // Poll 2: recovers
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await waitFor(() => {
        expect(screen.getAllByText("Completed").length).toBeGreaterThanOrEqual(1);
      });

      vi.useRealTimers();
    });

    it("stops polling and does not leak intervals on unmount", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });

      const pendingJob = buildExportJob({
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });

      const fetchMock = vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({ data: [pendingJob], total: 1, page: 1, pageSize: 50, hasMore: false }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      );
      global.fetch = fetchMock;

      const { unmount } = render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });

      // Record call count after initial fetch
      const callsBeforeUnmount = fetchMock.mock.calls.length;

      // Unmount while polling is active
      unmount();

      // Advance timers: no further fetches should happen
      await act(async () => {
        await vi.advanceTimersByTimeAsync(15000);
      });

      expect(fetchMock.mock.calls.length).toBe(callsBeforeUnmount);

      vi.useRealTimers();
    });
  });

  describe("error handling", () => {
    it("surfaces the failure once polling gives up", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });

      const pendingJob = buildExportJob({
        status: "pending",
        completedAt: null,
        fileSizeBytes: null,
      });

      const listResponse = () =>
        new Response(
          JSON.stringify({
            data: [pendingJob],
            total: 1,
            page: 1,
            pageSize: 50,
            hasMore: false,
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        );
      const fetchMock = vi
        .fn()
        .mockResolvedValueOnce(listResponse())
        .mockRejectedValue(new TypeError("Failed to fetch"));
      global.fetch = fetchMock;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
      });

      // Three consecutive failed polls exhaust the failure budget.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(15000);
      });

      await waitFor(() => {
        expect(
          screen.getByText(
            "Lost contact with the server while tracking your export. Refresh to see the latest status.",
          ),
        ).toBeInTheDocument();
      });

      // Polling stopped: no further requests.
      const callsAfterGivingUp = fetchMock.mock.calls.length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(20000);
      });
      expect(fetchMock.mock.calls.length).toBe(callsAfterGivingUp);

      vi.useRealTimers();
    });

    it("resolves loading when initial fetch throws non-ApiRequestError", async () => {
      // Simulates a raw network failure (not an HTTP error response)
      global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));

      render(<ExportDataSection />);

      // Loading resolves: component renders its main content (empty state)
      await waitFor(() => {
        expect(screen.getByText("No exports yet")).toBeInTheDocument();
      });

      // Export button should be enabled (no active jobs, no cooldown)
      expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
    });

    it("re-enables button when export creation throws non-ApiRequestError", async () => {
      const user = userEvent.setup();

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: empty list
          { status: 200, body: { data: [], total: 0, page: 1, pageSize: 50, hasMore: false } },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      // Replace fetch with one that throws a raw TypeError on POST
      global.fetch = vi.fn().mockRejectedValue(new TypeError("Network disconnected"));

      await user.click(screen.getByRole("button", { name: /export my data/i }));

      // Button should re-enable after the error (creating goes back to false)
      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      // No pending job should appear (creation failed)
      expect(screen.queryAllByText("Pending")).toHaveLength(0);
    });

    it("handles 429 with full ISO datetime in message (includes time component)", async () => {
      const user = userEvent.setup();
      const retryAfter = buildFutureRetryAfter();

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: empty list
          { status: 200, body: { data: [], total: 0, page: 1, pageSize: 50, hasMore: false } },
          // POST: rate limited with full ISO datetime in message
          {
            status: 429,
            body: {
              code: "RATE_LIMITED",
              message: `Export limit reached. Next available: ${retryAfter}`,
            },
          },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      await user.click(screen.getByRole("button", { name: /export my data/i }));

      await waitFor(() => {
        expect(screen.getByText(/next export available/i)).toBeInTheDocument();
      });

      expect(screen.getByRole("button", { name: /export my data/i })).toBeDisabled();
    });

    it("handles 429 with no date in message (no match for regex)", async () => {
      const user = userEvent.setup();

      global.fetch = createMockApi({
        "/api/datarights/exports": mockSequence([
          // Initial GET: empty list
          { status: 200, body: { data: [], total: 0, page: 1, pageSize: 50, hasMore: false } },
          // POST: rate limited with no date in message
          {
            status: 429,
            body: {
              code: "RATE_LIMITED",
              message: "Rate limit exceeded. Please try again later.",
            },
          },
        ]),
      }) as unknown as typeof fetch;

      render(<ExportDataSection />);

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      await user.click(screen.getByRole("button", { name: /export my data/i }));

      // Should NOT show cooldown (no date to extract)
      // Button should re-enable since no cooldown date was set
      await waitFor(() => {
        expect(screen.getByRole("button", { name: /export my data/i })).toBeEnabled();
      });

      // No "next export available" text should appear
      expect(screen.queryByText(/next export available/i)).not.toBeInTheDocument();
    });
  });
});
