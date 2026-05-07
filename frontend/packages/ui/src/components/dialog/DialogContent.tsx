import * as React from "react";
import { cn } from "@gofin/ui/lib/utils";

export function DialogContent({
  className,
  children,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-content"
      className={cn(
        "relative w-full max-w-lg rounded-xl bg-card p-6 text-card-foreground shadow-lg ring-1 ring-foreground/10",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
