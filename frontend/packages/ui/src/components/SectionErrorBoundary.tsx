import * as React from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";

interface SectionErrorBoundaryProps {
  /** Name of the section for user-facing fallback message. */
  sectionName?: string;
  /** Custom fallback to render instead of the default error card. */
  fallback?: React.ReactNode;
  children: React.ReactNode;
}

interface SectionErrorBoundaryState {
  hasError: boolean;
  /**
   * Diagnostics retained from componentDidCatch. Not render inputs: the fallback
   * stays generic so a render crash never leaks internals to a user.
   */
  error: Error | null;
  componentStack: string | null;
}

/**
 * Error boundary that wraps individual dashboard sections so a failure
 * in one section does not crash the entire page.
 *
 * Renders a contained error card with a retry (remount) button.
 */
class SectionErrorBoundary extends React.Component<
  SectionErrorBoundaryProps,
  SectionErrorBoundaryState
> {
  constructor(props: SectionErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null, componentStack: null };
  }

  static getDerivedStateFromError(): Partial<SectionErrorBoundaryState> {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    this.setState({
      error,
      componentStack: errorInfo.componentStack ?? null,
    });
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null, componentStack: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      const label = this.props.sectionName ?? "this section";

      return (
        <div
          role="alert"
          className="flex flex-col items-center justify-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-6 py-8 text-center"
        >
          <AlertTriangle className="size-8 text-destructive/70" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-destructive">
              Could not load {label}
            </p>
            <p className="text-xs text-muted-foreground">
              Something went wrong rendering this section.
            </p>
          </div>
          <button
            type="button"
            onClick={this.handleRetry}
            className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-medium transition-colors hover:bg-muted"
          >
            <RefreshCw className="size-3" />
            Try again
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}

export { SectionErrorBoundary };
export type { SectionErrorBoundaryProps };
