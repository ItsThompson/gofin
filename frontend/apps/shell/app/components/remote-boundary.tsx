import * as React from "react";
import { RemoteLoadError } from "@gofin/ui/components/remote-load-error";

interface RemoteBoundaryProps {
  /** Section name for the fallback UI. */
  sectionName: string;
  /** Suspense fallback shown while loading. */
  loadingFallback: React.ReactNode;
  children: React.ReactNode;
}

interface RemoteBoundaryState {
  hasError: boolean;
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
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): RemoteBoundaryState {
    return { hasError: true };
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
