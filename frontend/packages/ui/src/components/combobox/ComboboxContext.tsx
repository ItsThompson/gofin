import * as React from "react";

import type { ComboboxContextValue } from "./types";

export const ComboboxContext = React.createContext<ComboboxContextValue | null>(null);

export function useComboboxContext() {
  const context = React.useContext(ComboboxContext);

  if (!context) {
    throw new Error("Combobox primitives must be used within Combobox");
  }

  return context;
}
