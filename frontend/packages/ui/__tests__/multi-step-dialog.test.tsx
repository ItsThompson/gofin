import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  MultiStepDialog,
  MultiStepDialogContent,
  MultiStepDialogStep,
  useMultiStepDialog,
} from "../src/components/multi-step-dialog";

function StepWithNavigation({ label }: { label: string }) {
  const { currentStep, totalSteps, next, back } = useMultiStepDialog();
  return (
    <div>
      <p>{label}</p>
      <p data-testid="step-info">
        Step {currentStep + 1} of {totalSteps}
      </p>
      <button onClick={back}>Back</button>
      <button onClick={next}>Next</button>
    </div>
  );
}

function TestDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <MultiStepDialog open={open} onOpenChange={onOpenChange}>
      <MultiStepDialogContent>
        <MultiStepDialogStep>
          <StepWithNavigation label="Step 1 Content" />
        </MultiStepDialogStep>
        <MultiStepDialogStep>
          <StepWithNavigation label="Step 2 Content" />
        </MultiStepDialogStep>
        <MultiStepDialogStep>
          <StepWithNavigation label="Step 3 Content" />
        </MultiStepDialogStep>
      </MultiStepDialogContent>
    </MultiStepDialog>
  );
}

describe("MultiStepDialog", () => {
  it("renders only the first step when opened", () => {
    render(<TestDialog open={true} onOpenChange={() => {}} />);

    expect(screen.getByText("Step 1 Content")).toBeInTheDocument();
    expect(screen.queryByText("Step 2 Content")).not.toBeInTheDocument();
    expect(screen.queryByText("Step 3 Content")).not.toBeInTheDocument();
  });

  it("provides correct step info via context", () => {
    render(<TestDialog open={true} onOpenChange={() => {}} />);

    expect(screen.getByTestId("step-info")).toHaveTextContent("Step 1 of 3");
  });

  it("navigates forward on next()", async () => {
    const user = userEvent.setup();
    render(<TestDialog open={true} onOpenChange={() => {}} />);

    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByText("Step 2 Content")).toBeInTheDocument();
    expect(screen.queryByText("Step 1 Content")).not.toBeInTheDocument();
    expect(screen.getByTestId("step-info")).toHaveTextContent("Step 2 of 3");
  });

  it("navigates backward on back()", async () => {
    const user = userEvent.setup();
    render(<TestDialog open={true} onOpenChange={() => {}} />);

    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Step 2 Content")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByText("Step 1 Content")).toBeInTheDocument();
  });

  it("back from step 0 is a no-op", async () => {
    const user = userEvent.setup();
    render(<TestDialog open={true} onOpenChange={() => {}} />);

    await user.click(screen.getByRole("button", { name: "Back" }));

    expect(screen.getByText("Step 1 Content")).toBeInTheDocument();
    expect(screen.getByTestId("step-info")).toHaveTextContent("Step 1 of 3");
  });

  it("next from last step is a no-op", async () => {
    const user = userEvent.setup();
    render(<TestDialog open={true} onOpenChange={() => {}} />);

    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Step 3 Content")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Step 3 Content")).toBeInTheDocument();
    expect(screen.getByTestId("step-info")).toHaveTextContent("Step 3 of 3");
  });

  it("does not render content when closed", () => {
    render(<TestDialog open={false} onOpenChange={() => {}} />);

    expect(screen.queryByText("Step 1 Content")).not.toBeInTheDocument();
  });

  it("resets to step 1 when closed and reopened", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<TestDialog open={true} onOpenChange={() => {}} />);

    // Navigate to step 2
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Step 2 Content")).toBeInTheDocument();

    // Close the dialog
    rerender(<TestDialog open={false} onOpenChange={() => {}} />);
    expect(screen.queryByText("Step 2 Content")).not.toBeInTheDocument();

    // Reopen: should be back at step 1
    rerender(<TestDialog open={true} onOpenChange={() => {}} />);
    expect(screen.getByText("Step 1 Content")).toBeInTheDocument();
    expect(screen.getByTestId("step-info")).toHaveTextContent("Step 1 of 3");
  });
});
