import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { SectionErrorBoundary } from "../src/components/SectionErrorBoundary";

/**
 * A child component that throws on render when `shouldThrow` is true.
 * Used to trigger error boundaries in tests.
 */
function ThrowingChild({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error("Child render error");
  }
  return <div>Child rendered successfully</div>;
}

describe("SectionErrorBoundary", () => {
  beforeEach(() => {
    // Suppress React error boundary console.error noise
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  it("renders children normally when no error occurs", () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <div>Normal content</div>
      </SectionErrorBoundary>,
    );

    expect(screen.getByText("Normal content")).toBeInTheDocument();
  });

  it("catches render errors thrown by children", () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <ThrowingChild shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(
      screen.queryByText("Child rendered successfully"),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("retains the caught error and the component stack", () => {
    const didCatch = vi.spyOn(SectionErrorBoundary.prototype, "componentDidCatch");
    const boundary = React.createRef<SectionErrorBoundary>();

    render(
      <SectionErrorBoundary ref={boundary} sectionName="Budget">
        <ThrowingChild shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(didCatch).toHaveBeenCalledTimes(1);
    const [caughtError, errorInfo] = didCatch.mock.calls[0];
    expect(caughtError).toBeInstanceOf(Error);
    expect(caughtError.message).toBe("Child render error");
    expect(errorInfo.componentStack).toContain("ThrowingChild");

    expect(boundary.current?.state.error).toBe(caughtError);
    expect(boundary.current?.state.componentStack).toContain("ThrowingChild");

    // Capture only: the fallback is unchanged.
    expect(screen.getByText("Could not load Budget")).toBeInTheDocument();

    didCatch.mockRestore();
  });

  it("clears the retained diagnostics on retry", async () => {
    const user = userEvent.setup();
    const boundary = React.createRef<SectionErrorBoundary>();
    let shouldThrow = true;

    function ConditionalChild() {
      if (shouldThrow) {
        throw new Error("Temporary error");
      }
      return <div>Recovered content</div>;
    }

    render(
      <SectionErrorBoundary ref={boundary} sectionName="Budget">
        <ConditionalChild />
      </SectionErrorBoundary>,
    );

    expect(boundary.current?.state.error).not.toBeNull();

    shouldThrow = false;
    await user.click(screen.getByRole("button", { name: /try again/i }));

    expect(boundary.current?.state.error).toBeNull();
    expect(boundary.current?.state.componentStack).toBeNull();
  });

  it("renders a fallback UI displaying the error message with section name", () => {
    render(
      <SectionErrorBoundary sectionName="Budget">
        <ThrowingChild shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(screen.getByText("Could not load Budget")).toBeInTheDocument();
    expect(
      screen.getByText("Something went wrong rendering this section."),
    ).toBeInTheDocument();
  });

  it("renders a generic label when sectionName is not provided", () => {
    render(
      <SectionErrorBoundary>
        <ThrowingChild shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(
      screen.getByText("Could not load this section"),
    ).toBeInTheDocument();
  });

  it("renders custom fallback when provided", () => {
    render(
      <SectionErrorBoundary fallback={<div>Custom error fallback</div>}>
        <ThrowingChild shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(screen.getByText("Custom error fallback")).toBeInTheDocument();
    expect(
      screen.queryByText("Could not load this section"),
    ).not.toBeInTheDocument();
  });

  it("reset/retry mechanism clears the error state and re-renders children", async () => {
    const user = userEvent.setup();

    // Use a mutable ref to control whether the child throws.
    // After retry, we need the child to succeed.
    let shouldThrow = true;

    function ConditionalChild() {
      if (shouldThrow) {
        throw new Error("Temporary error");
      }
      return <div>Recovered content</div>;
    }

    render(
      <SectionErrorBoundary sectionName="Budget">
        <ConditionalChild />
      </SectionErrorBoundary>,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Could not load Budget")).toBeInTheDocument();

    // Fix the child before retry
    shouldThrow = false;

    const retryButton = screen.getByRole("button", { name: /try again/i });
    await user.click(retryButton);

    expect(screen.getByText("Recovered content")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
