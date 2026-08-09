import * as React from "react";
import { reportError } from "@gofin/api";
import { RemoteLoadError } from "@gofin/ui/components/RemoteLoadError";

interface RemoteBoundaryProps {
  /** Section name for the fallback UI. */
  sectionName: string;
  /** Suspense fallback shown while loading. */
  loadingFallback: React.ReactNode;
  children: React.ReactNode;
}

interface RemoteBoundaryState {
  hasError: boolean;
  /**
   * Diagnostics retained from componentDidCatch. Not render inputs: the
   * fallback is deliberately generic, and a load failure carries nothing worth
   * showing a user.
   */
  error: Error | null;
  componentStack: string | null;
}

/**
 * Combines Suspense (for loading) with an error boundary (for load failures)
 * around lazy-loaded remote modules. If the dynamic import rejects (network
 * error, remote unavailable), shows the RemoteLoadError fallback instead of
 * crashing the page.
 */
class RemoteBoundary extends React.Component<
  RemoteBoundaryProps,
  RemoteBoundaryState
> {
  constructor(props: RemoteBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null, componentStack: null };
  }

  static getDerivedStateFromError(): Partial<RemoteBoundaryState> {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    this.setState({
      error,
      componentStack: errorInfo.componentStack ?? null,
    });
    reportError(error, {
      kind: "network",
      op: "chunk.load",
      domain: "platform",
      // Chunk names are content-hashed and assets are served with maxAge 1h, so
      // every client still holding a stale manifest after a deploy fails here
      // with a different filename. One key collapses a bad deploy into one issue
      // instead of one per stale client.
      groupKey: "chunk_load_failed",
      groupExact: true,
      data: {
        sectionName: this.props.sectionName,
        componentStack: errorInfo.componentStack,
      },
    });
  }

  render() {
    if (this.state.hasError) {
      return <RemoteLoadError sectionName={this.props.sectionName} />;
    }

    return (
      <React.Suspense fallback={this.props.loadingFallback}>
        {this.props.children}
      </React.Suspense>
    );
  }
}

export { RemoteBoundary };
