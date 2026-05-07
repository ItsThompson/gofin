import * as React from "react";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { Dialog, DialogContent } from "@gofin/ui/components/dialog";

interface MultiStepDialogContextValue {
  currentStep: number;
  totalSteps: number;
  next: () => void;
  back: () => void;
  reset: () => void;
}

const MultiStepDialogContext = createContext<MultiStepDialogContextValue | null>(null);

/**
 * Hook to access multi-step dialog navigation from within step components.
 * Must be used inside a MultiStepDialogContent component.
 */
function useMultiStepDialog(): MultiStepDialogContextValue {
  const context = useContext(MultiStepDialogContext);
  if (!context) {
    throw new Error("useMultiStepDialog must be used within a MultiStepDialogContent");
  }
  return context;
}

interface MultiStepDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
}

/**
 * Multi-step dialog wrapper. Resets to step 0 when closed.
 * Built on top of the existing Dialog component.
 */
function MultiStepDialog({ open, onOpenChange, children }: MultiStepDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {children}
    </Dialog>
  );
}

interface MultiStepDialogContentProps {
  className?: string;
  children: React.ReactNode;
}

/**
 * Content area for multi-step dialogs. Manages step state and provides
 * navigation context to children. Only renders the active step;
 * non-step children (e.g., DialogHeader) render normally.
 */
function MultiStepDialogContent({ className, children }: MultiStepDialogContentProps) {
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

interface MultiStepDialogStepProps {
  children: React.ReactNode;
}

/**
 * Individual step within a multi-step dialog. Only rendered when active.
 */
function MultiStepDialogStep({ children }: MultiStepDialogStepProps) {
  return <>{children}</>;
}

export {
  MultiStepDialog,
  MultiStepDialogContent,
  MultiStepDialogStep,
  useMultiStepDialog,
};
