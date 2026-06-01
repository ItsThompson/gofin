import { Input } from "../input";
import { useComboboxContext } from "./ComboboxContext";
import type { ComboboxInputProps } from "./types";

export function ComboboxInput({ id, onBlur, onFocus, onKeyDown, ...props }: ComboboxInputProps) {
  const {
    inputId,
    listboxId,
    isOpen,
    highlightedId,
    setIsOpen,
    setHighlightedId,
    getSelectableOptions,
    getHighlightedOption,
  } = useComboboxContext();
  const resolvedId = id ?? inputId;

  function moveHighlight(direction: 1 | -1) {
    const options = getSelectableOptions();

    if (options.length === 0) {
      setHighlightedId(null);
      return;
    }

    const currentIndex = highlightedId
      ? options.findIndex((option) => option.id === highlightedId)
      : -1;
    const nextIndex = currentIndex === -1
      ? direction === 1
        ? 0
        : options.length - 1
      : (currentIndex + direction + options.length) % options.length;

    setIsOpen(true);
    setHighlightedId(options[nextIndex].id);
  }

  return (
    <Input
      id={resolvedId}
      role="combobox"
      aria-autocomplete="list"
      aria-controls={listboxId}
      aria-expanded={isOpen}
      aria-activedescendant={highlightedId ?? undefined}
      onFocus={(event) => {
        setIsOpen(true);
        onFocus?.(event);
      }}
      onBlur={(event) => {
        setIsOpen(false);
        onBlur?.(event);
      }}
      onKeyDown={(event) => {
        if (event.key === "ArrowDown") {
          event.preventDefault();
          moveHighlight(1);
        } else if (event.key === "ArrowUp") {
          event.preventDefault();
          moveHighlight(-1);
        } else if (event.key === "Enter") {
          const highlightedOption = getHighlightedOption();

          if (highlightedOption && !highlightedOption.disabled) {
            event.preventDefault();
            highlightedOption.onSelect();
            if (highlightedOption.closeOnSelect) {
              setIsOpen(false);
            }
          }
        } else if (event.key === "Escape") {
          event.preventDefault();
          setIsOpen(false);
        }

        onKeyDown?.(event);
      }}
      {...props}
    />
  );
}
