import { Dialog } from "@gofin/ui/components/dialog";
import type { MultiStepDialogProps } from "./types";

/**
 * Multi-step dialog wrapper. Resets to step 0 when closed.
 * Built on top of the existing Dialog component.
 */
export function MultiStepDialog({ open, onOpenChange, children }: MultiStepDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {children}
    </Dialog>
  );
}
