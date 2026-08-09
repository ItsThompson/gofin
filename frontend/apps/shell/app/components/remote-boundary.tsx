import * as React from "react";
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
