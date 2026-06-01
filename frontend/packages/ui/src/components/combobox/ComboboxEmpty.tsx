import { cn } from "@gofin/ui/lib/utils";

import type { ComboboxEmptyProps } from "./types";

export function ComboboxEmpty({ className, ...props }: ComboboxEmptyProps) {
  return (
    <div
      role="presentation"
      className={cn("px-2 py-1.5 text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}
