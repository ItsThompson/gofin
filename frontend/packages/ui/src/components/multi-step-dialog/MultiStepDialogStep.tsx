import type { MultiStepDialogStepProps } from "./types";

/**
 * Individual step within a multi-step dialog. Only rendered when active.
 */
export function MultiStepDialogStep({ children }: MultiStepDialogStepProps) {
  return <>{children}</>;
}
