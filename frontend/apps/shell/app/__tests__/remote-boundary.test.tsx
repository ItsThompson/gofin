import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { RemoteBoundary } from "@/components/remote-boundary";

describe("RemoteBoundary", () => {
  const reloadMock = vi.fn();

  beforeEach(() => {
    // Suppress React error boundary console.error noise
    vi.spyOn(console, "error").mockImplementation(() => {});
    Object.defineProperty(window, "location", {
      value: { reload: reloadMock },
      writable: true,
    });
  });

  afterEach(() => {
    reloadMock.mockReset();
  });

  it("shows the loading fallback while the lazy chunk loads", async () => {
    // Create a lazy component that stays pending (never resolves during this test)
    const LazyComponent = React.lazy(
      () => new Promise<{ default: React.ComponentType }>(() => {}),
    );

    render(
      <RemoteBoundary
        sectionName="Dashboard"
        loadingFallback={<div>Loading spinner...</div>}
      >
        <LazyComponent />
      </RemoteBoundary>,
    );

    expect(screen.getByText("Loading spinner...")).toBeInTheDocument();
  });

  it("shows RemoteLoadError component when the chunk fails to load", async () => {
    // Create a lazy component that rejects immediately
    const LazyComponent = React.lazy(
      () => Promise.reject(new Error("Failed to fetch chunk")),
    );

    render(
      <RemoteBoundary
        sectionName="Dashboard"
        loadingFallback={<div>Loading...</div>}
      >
        <LazyComponent />
      </RemoteBoundary>,
    );

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });

    expect(screen.getByText("Could not load Dashboard")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Try refreshing the page. If the problem persists, check your connection.",
      ),
    ).toBeInTheDocument();
  });

  it("clicking retry in RemoteLoadError triggers a page reload", async () => {
    const user = userEvent.setup();

    const LazyComponent = React.lazy(
      () => Promise.reject(new Error("Network error")),
    );

    render(
      <RemoteBoundary
        sectionName="Settings"
        loadingFallback={<div>Loading...</div>}
      >
        <LazyComponent />
      </RemoteBoundary>,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /refresh page/i }),
      ).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /refresh page/i }));

    expect(reloadMock).toHaveBeenCalledTimes(1);
  });

  it("renders children successfully when the chunk loads", async () => {
    function SuccessContent() {
      return <div>Remote module loaded!</div>;
    }

    // Create a lazy component that resolves with a valid module
    const LazyComponent = React.lazy(
      () => Promise.resolve({ default: SuccessContent }),
    );

    render(
      <RemoteBoundary
        sectionName="Dashboard"
        loadingFallback={<div>Loading...</div>}
      >
        <LazyComponent />
      </RemoteBoundary>,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Remote module loaded!"),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Loading...")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the loading fallback before resolving, then renders the child", async () => {
    let resolveImport!: (
      value: { default: React.ComponentType },
    ) => void;

    function LoadedContent() {
      return <div>Content loaded</div>;
    }

    const LazyComponent = React.lazy(
      () =>
        new Promise<{ default: React.ComponentType }>((resolve) => {
          resolveImport = resolve;
        }),
    );

    render(
      <RemoteBoundary
        sectionName="Admin"
        loadingFallback={<div>Loading admin...</div>}
      >
        <LazyComponent />
      </RemoteBoundary>,
    );

    // Initially shows loading state
    expect(screen.getByText("Loading admin...")).toBeInTheDocument();
    expect(screen.queryByText("Content loaded")).not.toBeInTheDocument();

    // Resolve the lazy import
    await act(async () => {
      resolveImport({ default: LoadedContent });
    });

    // Now shows the resolved content
    await waitFor(() => {
      expect(screen.getByText("Content loaded")).toBeInTheDocument();
    });
    expect(screen.queryByText("Loading admin...")).not.toBeInTheDocument();
  });
});
