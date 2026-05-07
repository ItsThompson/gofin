import { useContext } from "react";
import { MultiStepDialogContext } from "./context";
import type { MultiStepDialogContextValue } from "./types";

/**
 * Hook to access multi-step dialog navigation from within step components.
 * Must be used inside a MultiStepDialogContent component.
 */
export function useMultiStepDialog(): MultiStepDialogContextValue {
  const context = useContext(MultiStepDialogContext);
  if (!context) {
    throw new Error("useMultiStepDialog must be used within a MultiStepDialogContent");
  }
  return context;
}
