import type { ReactNode } from "react";

export interface MultiStepDialogContextValue {
  currentStep: number;
  totalSteps: number;
  next: () => void;
  back: () => void;
  reset: () => void;
}

export interface MultiStepDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
}

export interface MultiStepDialogContentProps {
  className?: string;
  children: ReactNode;
}

export interface MultiStepDialogStepProps {
  children: ReactNode;
}
