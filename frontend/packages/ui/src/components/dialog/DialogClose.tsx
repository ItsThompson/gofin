import * as React from "react";
import { cn } from "@gofin/ui/lib/utils";

export function DialogClose({
  className,
  onClick,
  ...props
}: React.ComponentProps<"button">) {
  return (
    <button
      type="button"
      data-slot="dialog-close"
      className={cn(
        "rounded-sm p-1 opacity-70 transition-opacity hover:opacity-100",
        className,
      )}
      aria-label="Close"
      onClick={onClick}
      {...props}
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M18 6 6 18" />
        <path d="m6 6 12 12" />
      </svg>
    </button>
  );
}
