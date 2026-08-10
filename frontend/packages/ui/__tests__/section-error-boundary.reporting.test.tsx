import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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

import { SectionErrorBoundary } from "../src/components/SectionErrorBoundary";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

function ThrowingWidget(): React.ReactNode {
  throw new Error("widget render crash");
}

describe("SectionErrorBoundary reporting", () => {
  beforeEach(() => {
    captureException.mockClear();
    // React logs every boundary catch; the assertions are on the capture.
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  it("reports a throwing child exactly once", async () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <ThrowingWidget />
      </SectionErrorBoundary>,
    );

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    const { error, context } = onlyCapture();
    expect((error as Error).message).toBe("widget render crash");
    expect(context.tags).toMatchObject({
      error_kind: "internal",
      operation: "render.section",
      domain: "platform",
    });
  });

  it("carries the section name and the component stack in the gofin block", async () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <ThrowingWidget />
      </SectionErrorBoundary>,
    );

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    const gofin = onlyCapture().context.contexts?.gofin;
    expect(gofin?.sectionName).toBe("Budget");
    expect(gofin?.componentStack).toContain("ThrowingWidget");
  });

  it("keeps the fallback generic, so nothing internal reaches the user", async () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <ThrowingWidget />
      </SectionErrorBoundary>,
    );

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    expect(screen.getByText("Could not load Budget")).toBeInTheDocument();
    expect(screen.queryByText(/widget render crash/)).not.toBeInTheDocument();
  });

  it("reports nothing when the children render", () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <div>Normal content</div>
      </SectionErrorBoundary>,
    );

    expect(captureException).not.toHaveBeenCalled();
  });
});
