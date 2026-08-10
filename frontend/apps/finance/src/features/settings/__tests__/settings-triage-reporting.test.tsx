import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { createMockApi } from "@gofin/test-utils";

/** The subset of Sentry's CaptureContext that reportError sends. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException, toastError } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
  toastError: vi.fn(),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));
vi.mock("sonner", () => ({
  toast: { error: toastError, success: vi.fn() },
}));

import { TagsSection } from "../components/TagsSection";
import { ExportDataSection } from "../components/ExportDataSection";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

beforeEach(() => {
  captureException.mockClear();
  toastError.mockClear();
});

describe("the tags list", () => {
  it("reports a failed load and says so, instead of claiming there are no tags", async () => {
    global.fetch = createMockApi({
      "/api/finance/tags": { status: 503, body: { code: "UPSTREAM" } },
    }) as unknown as typeof fetch;

    render(<TagsSection />);

    await waitFor(() =>
      expect(
        screen.getByText("Could not load your tags. Refresh to try again."),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText("No tags yet. Add one above.")).toBeNull();

    expect(onlyCapture().context.tags).toMatchObject({
      error_kind: "upstream",
      operation: "tag.list",
      domain: "expenses",
    });
  });

  it("still says there are no tags when the load succeeds and returns none", async () => {
    global.fetch = createMockApi({
      "/api/finance/tags": { body: { tags: [] } },
    }) as unknown as typeof fetch;

    render(<TagsSection />);

    await waitFor(() =>
      expect(screen.getByText("No tags yet. Add one above.")).toBeInTheDocument(),
    );
    expect(captureException).not.toHaveBeenCalled();
  });
});

describe("the export status poll", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("reports the terminal failure once, from the caller", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const listResponse = () =>
      new Response(
        JSON.stringify({
          data: [
            {
              id: "job-1",
              userId: "user-1",
              status: "pending",
              createdAt: new Date().toISOString(),
              completedAt: null,
              fileSizeBytes: null,
            },
          ],
          total: 1,
          page: 1,
          pageSize: 50,
          hasMore: false,
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      );
    global.fetch = vi
      .fn()
      .mockResolvedValueOnce(listResponse())
      .mockRejectedValue(
        new TypeError("Failed to fetch"),
      ) as unknown as typeof fetch;

    render(<ExportDataSection />);
    await waitFor(() =>
      expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1),
    );

    // Three consecutive failures exhaust the budget. The extra ticks would each
    // add an event if the transport layer reported per failure.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60000);
    });

    const { context } = onlyCapture();
    expect(context.tags).toMatchObject({
      error_kind: "network",
      operation: "datarights.export_status",
      domain: "datarights",
    });
    // A give-up is outage-grade even after a 4xx, so it is never dropped.
    expect(context.tags?.expected).toBeUndefined();
    expect(context.level).toBe("error");
  });
});
