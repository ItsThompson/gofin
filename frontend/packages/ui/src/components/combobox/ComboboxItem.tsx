import * as React from "react";

import { cn } from "@gofin/ui/lib/utils";

import { useComboboxContext } from "./ComboboxContext";
import type { ComboboxItemProps } from "./types";

export function ComboboxItem({
  value,
  disabled = false,
  closeOnSelect = true,
  onSelect,
  className,
  children,
  ...props
}: ComboboxItemProps) {
  const itemId = React.useId();
  const { highlightedId, setHighlightedId, setIsOpen, registerOption, unregisterOption } =
    useComboboxContext();
  const isHighlighted = highlightedId === itemId;

  const selectItem = React.useCallback(() => {
    if (disabled) {
      return;
    }

    onSelect?.(value);
    if (closeOnSelect) {
      setIsOpen(false);
    }
  }, [closeOnSelect, disabled, onSelect, setIsOpen, value]);

  React.useEffect(() => {
    registerOption({ id: itemId, value, disabled, closeOnSelect, onSelect: selectItem });

    return () => unregisterOption(itemId);
  }, [closeOnSelect, disabled, itemId, registerOption, selectItem, unregisterOption, value]);

  return (
    <div
      id={itemId}
      role="option"
      aria-disabled={disabled || undefined}
      aria-selected={isHighlighted}
      data-highlighted={isHighlighted ? "true" : undefined}
      className={cn(
        "cursor-default rounded-sm px-2 py-1.5 text-sm outline-none",
        isHighlighted && "bg-accent text-accent-foreground",
        disabled && "pointer-events-none opacity-50",
        className,
      )}
      onMouseDown={(event) => {
        event.preventDefault();
      }}
      onMouseEnter={() => {
        if (!disabled) {
          setHighlightedId(itemId);
        }
      }}
      onClick={selectItem}
      {...props}
    >
      {children}
    </div>
  );
}
