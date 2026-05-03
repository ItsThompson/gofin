import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SectionErrorBoundary } from "@gofin/ui/components/section-error-boundary";

// Suppress React error boundary console.error noise in tests
const originalConsoleError = console.error;
beforeEach(() => {
  console.error = vi.fn();
});

afterEach(() => {
  console.error = originalConsoleError;
});

function ThrowingComponent({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error("Test rendering error");
  }
  return <div>Content rendered successfully</div>;
}

describe("SectionErrorBoundary", () => {
  it("renders children when no error occurs", () => {
    render(
      <SectionErrorBoundary sectionName="Test Section">
        <div>Normal content</div>
      </SectionErrorBoundary>,
    );

    expect(screen.getByText("Normal content")).toBeInTheDocument();
  });

  it("renders error fallback when child throws", () => {
    render(
      <SectionErrorBoundary sectionName="Dashboard Widget">
        <ThrowingComponent shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      screen.getByText("Could not load Dashboard Widget"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Something went wrong rendering this section."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /try again/i }),
    ).toBeInTheDocument();
  });

  it("uses default section name when not provided", () => {
    render(
      <SectionErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(
      screen.getByText("Could not load this section"),
    ).toBeInTheDocument();
  });

  it("renders custom fallback when provided", () => {
    render(
      <SectionErrorBoundary
        fallback={<div>Custom error UI</div>}
      >
        <ThrowingComponent shouldThrow={true} />
      </SectionErrorBoundary>,
    );

    expect(screen.getByText("Custom error UI")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("recovers on retry click by remounting children", async () => {
    let throwOnRender = true;

    function ConditionalThrower() {
      if (throwOnRender) {
        throw new Error("Fail first time");
      }
      return <div>Recovered content</div>;
    }

    render(
      <SectionErrorBoundary sectionName="Widget">
        <ConditionalThrower />
      </SectionErrorBoundary>,
    );

    expect(screen.getByText("Could not load Widget")).toBeInTheDocument();

    // Stop throwing before clicking retry
    throwOnRender = false;

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /try again/i }));

    expect(screen.getByText("Recovered content")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
