import * as React from "react";
import { cn } from "@gofin/ui/lib/utils";

export function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("mb-4 flex items-start justify-between gap-4", className)}
      {...props}
    />
  );
}
