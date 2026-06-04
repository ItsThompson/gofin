import * as React from "react";

import { ComboboxContext } from "./ComboboxContext";
import type { ComboboxOptionRegistration, ComboboxProps } from "./types";

export function Combobox({ children, open, defaultOpen = false, onOpenChange }: ComboboxProps) {
  const inputId = React.useId();
  const listboxId = React.useId();
  const optionsRef = React.useRef(new Map<string, ComboboxOptionRegistration>());
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(defaultOpen);
  const [highlightedId, setHighlightedId] = React.useState<string | null>(null);
  const isOpen = open ?? uncontrolledOpen;

  const setIsOpen = React.useCallback(
    (nextOpen: boolean) => {
      if (open === undefined) {
        setUncontrolledOpen(nextOpen);
      }

      onOpenChange?.(nextOpen);

      if (!nextOpen) {
        setHighlightedId(null);
      }
    },
    [onOpenChange, open],
  );

  const registerOption = React.useCallback((option: ComboboxOptionRegistration) => {
    optionsRef.current.set(option.id, option);
  }, []);

  const unregisterOption = React.useCallback((id: string) => {
    optionsRef.current.delete(id);
    setHighlightedId((currentId) => (currentId === id ? null : currentId));
  }, []);

  const getSelectableOptions = React.useCallback(
    () => Array.from(optionsRef.current.values()).filter((option) => !option.disabled),
    [],
  );

  const getHighlightedOption = React.useCallback(() => {
    if (!highlightedId) {
      return null;
    }

    return optionsRef.current.get(highlightedId) ?? null;
  }, [highlightedId]);

  const value = React.useMemo(
    () => ({
      inputId,
      listboxId,
      isOpen,
      highlightedId,
      setIsOpen,
      setHighlightedId,
      registerOption,
      unregisterOption,
      getSelectableOptions,
      getHighlightedOption,
    }),
    [
      getHighlightedOption,
      getSelectableOptions,
      highlightedId,
      inputId,
      isOpen,
      listboxId,
      registerOption,
      setIsOpen,
      unregisterOption,
    ],
  );

  return <ComboboxContext.Provider value={value}>{children}</ComboboxContext.Provider>;
}
