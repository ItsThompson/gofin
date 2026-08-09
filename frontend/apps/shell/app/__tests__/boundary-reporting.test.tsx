import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import * as React from "react";

/** The subset of Sentry's CaptureContext that reportError sends. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));

import { ErrorBoundary } from "../root";
import { RemoteBoundary } from "@/components/remote-boundary";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

/**
 * The route boundary takes the framework's generated props, of which only `error`
 * is read. One boundary cast keeps the rest out of every call site.
 */
function routeBoundary(error: unknown): React.ReactElement {
  return React.createElement(
    ErrorBoundary,
    { error } as React.ComponentProps<typeof ErrorBoundary>,
  );
}

function ThrowingChild(): React.ReactNode {
  throw new Error("widget render crash");
}

describe("the root route ErrorBoundary", () => {
  beforeEach(() => {
    captureException.mockClear();
  });

  it("reports the error once, from an effect", async () => {
    render(routeBoundary(new Error("route render crash")));

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    const { error, context } = onlyCapture();
    expect((error as Error).message).toBe("route render crash");
    expect(context.tags).toMatchObject({
      error_kind: "internal",
      operation: "render.route",
      domain: "platform",
    });
  });

  it("reports once across a re-render with the same error", async () => {
    const error = new Error("route render crash");
    const { rerender } = render(routeBoundary(error));

    await waitFor(() => expect(captureException).toHaveBeenCalled());
    rerender(routeBoundary(error));

    onlyCapture();
  });

  it("reports again for a different error", async () => {
    const { rerender } = render(routeBoundary(new Error("first")));

    await waitFor(() => expect(captureException).toHaveBeenCalledTimes(1));
    rerender(routeBoundary(new Error("second")));

    await waitFor(() => expect(captureException).toHaveBeenCalledTimes(2));
  });

  it("reports nothing during a server render", () => {
    // Effects do not run on the server, which is what makes entry.server.tsx's
    // handleError the sole SSR owner with no origin test to write.
    const html = renderToString(routeBoundary(new Error("ssr render crash")));

    expect(html).toContain("Oops!");
    expect(captureException).not.toHaveBeenCalled();
  });

  it("reports nothing for a 404 route error", async () => {
    render(
      routeBoundary({
        status: 404,
        statusText: "Not Found",
        internal: false,
        data: null,
      }),
    );

    await waitFor(() =>
      expect(document.body.textContent).toContain(
        "The requested page could not be found.",
      ),
    );
    expect(captureException).not.toHaveBeenCalled();
  });

  it("reports a non-404 route error response", async () => {
    render(
      routeBoundary({
        status: 500,
        statusText: "Internal Server Error",
        internal: false,
        data: null,
      }),
    );

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    onlyCapture();
  });

  it("keeps the stack away from users in production", () => {
    vi.stubEnv("DEV", false);

    render(routeBoundary(new Error("leaky internals")));

    expect(document.body.textContent).toContain("An unexpected error occurred.");
    expect(document.body.textContent).not.toContain("leaky internals");
    expect(document.querySelector("pre")).toBeNull();

    vi.unstubAllEnvs();
  });

  it("shows the stack in development", () => {
    vi.stubEnv("DEV", true);

    render(routeBoundary(new Error("leaky internals")));

    expect(document.body.textContent).toContain("leaky internals");

    vi.unstubAllEnvs();
  });
});

describe("RemoteBoundary", () => {
  beforeEach(() => {
    captureException.mockClear();
    // React logs every boundary catch; the assertions are on the capture.
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("reports once with the component stack and one grouping key", async () => {
    render(
      <RemoteBoundary sectionName="Admin Panel" loadingFallback={<div />}>
        <ThrowingChild />
      </RemoteBoundary>,
    );

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    const { error, context } = onlyCapture();
    expect((error as Error).message).toBe("widget render crash");
    expect(context.tags).toMatchObject({
      error_kind: "network",
      operation: "chunk.load",
      domain: "platform",
    });
    expect(context.contexts?.gofin.sectionName).toBe("Admin Panel");
    expect(context.contexts?.gofin.componentStack).toContain("ThrowingChild");
  });

  it("collapses every load failure into one issue", async () => {
    // Chunk filenames are content-hashed, so each stale client after a deploy
    // fails on a different filename. Dropping "{{ default }}" is what keeps one
    // bad deploy at one issue instead of one per client.
    const LazyRemote = React.lazy(() =>
      Promise.reject(new Error("Failed to fetch dynamically imported module")),
    );

    render(
      <RemoteBoundary sectionName="Finance" loadingFallback={<div />}>
        <LazyRemote />
      </RemoteBoundary>,
    );

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    expect(onlyCapture().context.fingerprint).toEqual(["chunk_load_failed"]);
  });
});
