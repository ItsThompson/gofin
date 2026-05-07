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
  children: React.ReactNode;
}

export interface MultiStepDialogContentProps {
  className?: string;
  children: React.ReactNode;
}

export interface MultiStepDialogStepProps {
  children: React.ReactNode;
}
