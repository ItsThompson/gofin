import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SectionState } from "../components/SectionState";

function renderState(
  state: Parameters<typeof SectionState<string>>[0]["state"],
  onRetry = vi.fn(),
) {
  return render(
    <SectionState
      label="Summary"
      state={state}
      onRetry={onRetry}
      emptyMessage="Nothing returned."
    >
      {(value) => <p>{value}</p>}
    </SectionState>,
  );
}

describe("SectionState", () => {
  it("renders a stable loading placeholder", () => {
    renderState({ status: "loading" });
    expect(screen.getByLabelText("Summary loading")).toBeInTheDocument();
  });

  it("renders an explicit error and retries only that section", () => {
    const onRetry = vi.fn();
    renderState(
      { status: "error", error: { message: "Summary failed" } },
      onRetry,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Summary failed");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("renders valid empty results without invoking the success renderer", () => {
    renderState({ status: "empty" });
    expect(screen.getByText("Nothing returned.")).toBeInTheDocument();
    expect(screen.queryByText("undefined")).not.toBeInTheDocument();
  });
});
