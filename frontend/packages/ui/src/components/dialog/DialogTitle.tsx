import * as React from "react";
import { cn } from "@gofin/ui/lib/utils";

export function DialogTitle({ className, ...props }: React.ComponentProps<"h2">) {
  return (
    <h2
      data-slot="dialog-title"
      className={cn("text-lg font-semibold", className)}
      {...props}
    />
  );
}
