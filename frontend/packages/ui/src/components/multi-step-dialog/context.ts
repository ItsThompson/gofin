import { createContext } from "react";
import type { MultiStepDialogContextValue } from "./types";

export const MultiStepDialogContext = createContext<MultiStepDialogContextValue | null>(null);
