import type * as React from "react";

export interface ComboboxOptionRegistration {
  id: string;
  value: string;
  disabled: boolean;
  closeOnSelect: boolean;
  onSelect: () => void;
}

export interface ComboboxContextValue {
  inputId: string;
  listboxId: string;
  isOpen: boolean;
  highlightedId: string | null;
  setIsOpen: (isOpen: boolean) => void;
  setHighlightedId: (id: string | null) => void;
  registerOption: (option: ComboboxOptionRegistration) => void;
  unregisterOption: (id: string) => void;
  getSelectableOptions: () => ComboboxOptionRegistration[];
  getHighlightedOption: () => ComboboxOptionRegistration | null;
}

export interface ComboboxProps {
  children: React.ReactNode;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export type ComboboxInputProps = React.ComponentPropsWithoutRef<"input">;
export type ComboboxContentProps = React.ComponentPropsWithoutRef<"div">;
export type ComboboxListProps = React.ComponentPropsWithoutRef<"div">;
export type ComboboxEmptyProps = React.ComponentPropsWithoutRef<"div">;

export interface ComboboxItemProps extends Omit<React.ComponentPropsWithoutRef<"div">, "onSelect"> {
  value: string;
  disabled?: boolean;
  closeOnSelect?: boolean;
  onSelect?: (value: string) => void;
}
