import { cn } from "@gofin/ui/lib/utils";

import { useComboboxContext } from "./ComboboxContext";
import type { ComboboxListProps } from "./types";

export function ComboboxList({ className, ...props }: ComboboxListProps) {
  const { listboxId } = useComboboxContext();

  return (
    <div
      id={listboxId}
      role="listbox"
      className={cn("max-h-72 overflow-auto", className)}
      {...props}
    />
  );
}
