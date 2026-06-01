import { cn } from "@gofin/ui/lib/utils";

import { useComboboxContext } from "./ComboboxContext";
import type { ComboboxContentProps } from "./types";

export function ComboboxContent({ className, children, ...props }: ComboboxContentProps) {
  const { isOpen } = useComboboxContext();

  if (!isOpen) {
    return null;
  }

  return (
    <div
      className={cn(
        "mt-1 rounded-md border bg-popover p-1 text-popover-foreground shadow-md",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
