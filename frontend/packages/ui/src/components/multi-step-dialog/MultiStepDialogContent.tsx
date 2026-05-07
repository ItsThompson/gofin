import * as React from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { DialogContent } from "@gofin/ui/components/dialog";
import { MultiStepDialogContext } from "./context";
import { MultiStepDialogStep } from "./MultiStepDialogStep";
import type { MultiStepDialogContentProps } from "./types";

/**
 * Content area for multi-step dialogs. Manages step state and provides
 * navigation context to children. Only renders the active step;
 * non-step children (e.g., DialogHeader) render normally.
 */
export function MultiStepDialogContent({ className, children }: MultiStepDialogContentProps) {
  const [currentStep, setCurrentStep] = useState(0);

  const childArray = React.Children.toArray(children);

  const steps = childArray.filter(
    (child) => React.isValidElement(child) && child.type === MultiStepDialogStep,
  );

  const nonStepChildren = childArray.filter(
    (child) => !(React.isValidElement(child) && child.type === MultiStepDialogStep),
  );

  const totalSteps = steps.length;

  const next = useCallback(() => {
    setCurrentStep((prev) => Math.min(prev + 1, totalSteps - 1));
  }, [totalSteps]);

  const back = useCallback(() => {
    setCurrentStep((prev) => Math.max(prev - 1, 0));
  }, []);

  const reset = useCallback(() => {
    setCurrentStep(0);
  }, []);

  // Reset when the dialog content unmounts (which happens when Dialog closes)
  useEffect(() => {
    return () => {
      setCurrentStep(0);
    };
  }, []);

  const contextValue = useMemo(
    () => ({ currentStep, totalSteps, next, back, reset }),
    [currentStep, totalSteps, next, back, reset],
  );

  return (
    <MultiStepDialogContext.Provider value={contextValue}>
      <DialogContent className={className}>
        {nonStepChildren}
        {steps[currentStep]}
      </DialogContent>
    </MultiStepDialogContext.Provider>
  );
}
